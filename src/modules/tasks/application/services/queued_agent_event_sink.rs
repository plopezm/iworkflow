use std::sync::Arc;

use tokio::sync::{mpsc, oneshot};
use tokio::task::JoinHandle;

use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;

enum QueueItem {
    Event(TaskId, AgentEvent),
    /// Marcador de sincronización: el worker responde al llegar a él, es decir,
    /// tras procesar todo lo encolado antes.
    Flush(oneshot::Sender<()>),
}

type QueueSender = mpsc::Sender<QueueItem>;
type QueueReceiver = mpsc::Receiver<QueueItem>;

pub const DEFAULT_QUEUE_CAPACITY: usize = 4096;

/// Decorador de `AgentEventSink` que desacopla al lector de stdout del runtime
/// de la base de datos: `handle` solo encola y un único worker consume la cola
/// en orden FIFO (lo que preserva el orden por tarea).
///
/// Si la cola se llena, `handle` espera (backpressure): ningún evento se
/// descarta y la memoria queda acotada. Un pánico al procesar un evento queda
/// aislado en ese evento. Si el worker ha muerto, el evento se procesa en línea
/// para no perderlo.
///
/// Los eventos que aún estén en memoria se pierden si el proceso muere de
/// forma abrupta; en un apagado ordenado `shutdown` los drena todos.
pub struct QueuedAgentEventSink {
    sender: QueueSender,
    worker: JoinHandle<()>,
    inner: Arc<dyn AgentEventSink>,
}

impl QueuedAgentEventSink {
    /// Requiere un runtime de tokio activo (lanza el worker).
    pub fn new(inner: Arc<dyn AgentEventSink>, capacity: usize) -> Self {
        let (sender, mut receiver): (QueueSender, QueueReceiver) = mpsc::channel(capacity.max(1));
        let worker_sink: Arc<dyn AgentEventSink> = Arc::clone(&inner);
        let worker: JoinHandle<()> = tokio::spawn(async move {
            while let Some(item) = receiver.recv().await {
                match item {
                    QueueItem::Event(task_id, event) => {
                        // Cada evento corre en su propia tarea: un pánico solo pierde
                        // ese evento y no arrastra al resto de la cola ni al worker.
                        let sink: Arc<dyn AgentEventSink> = Arc::clone(&worker_sink);
                        let result: Result<(), tokio::task::JoinError> =
                            tokio::spawn(async move { sink.handle(task_id, event).await }).await;
                        if let Err(error) = result {
                            tracing::error!(
                                %task_id,
                                %error,
                                "el procesado de un evento de agente falló"
                            );
                        }
                    }
                    QueueItem::Flush(done) => {
                        // Si quien espera ya no está, no hay nadie a quien avisar.
                        let _ = done.send(());
                    }
                }
            }
        });
        Self {
            sender,
            worker,
            inner,
        }
    }

    /// Espera a que se hayan procesado todos los eventos aceptados hasta ahora.
    ///
    /// Encola un marcador (esperando hueco, sin descartar) y aguarda a que el
    /// worker lo alcance. Si el worker ya no existe, los eventos se procesaron
    /// en línea y retorna de inmediato.
    pub async fn flush(&self) {
        let (done, completed): (oneshot::Sender<()>, oneshot::Receiver<()>) = oneshot::channel();
        if self.sender.send(QueueItem::Flush(done)).await.is_err() {
            return;
        }
        // Un error aquí significa que el worker murió antes de responder.
        let _ = completed.await;
    }

    /// Cierra la cola y espera a que el worker procese todo lo encolado.
    pub async fn shutdown(self) {
        let Self { sender, worker, .. } = self;
        drop(sender);
        if let Err(error) = worker.await {
            tracing::error!(%error, "el worker de eventos de agente terminó con error");
        }
    }
}

#[async_trait::async_trait]
impl AgentEventSink for QueuedAgentEventSink {
    async fn handle(&self, task_id: TaskId, event: AgentEvent) {
        if let Err(mpsc::error::SendError(QueueItem::Event(task_id, event))) =
            self.sender.send(QueueItem::Event(task_id, event)).await
        {
            tracing::error!(
                task_id = %task_id,
                "worker de eventos de agente caído; procesando el evento en línea"
            );
            self.inner.handle(task_id, event).await;
        }
    }
}

#[cfg(test)]
mod tests {
    use std::time::Duration;

    use tokio::sync::Semaphore;

    use super::*;

    type Log = Arc<std::sync::Mutex<Vec<(TaskId, String)>>>;

    const PANIC_MARKER: &str = "panic";

    fn event(text: &str) -> AgentEvent {
        AgentEvent::UserMessage {
            text: text.to_string(),
            images: 0,
        }
    }

    struct RecordingSink {
        log: Log,
        delay: Option<Duration>,
        gate: Option<Arc<Semaphore>>,
    }

    impl RecordingSink {
        fn new(log: Log) -> Self {
            Self {
                log,
                delay: None,
                gate: None,
            }
        }
    }

    #[async_trait::async_trait]
    impl AgentEventSink for RecordingSink {
        async fn handle(&self, task_id: TaskId, event: AgentEvent) {
            if let Some(gate) = &self.gate
                && let Ok(permit) = gate.acquire().await
            {
                permit.forget();
            }
            if let Some(delay) = self.delay {
                tokio::time::sleep(delay).await;
            }
            let AgentEvent::UserMessage { text, .. } = event else {
                return;
            };
            if text == PANIC_MARKER {
                panic!("marker");
            }
            self.log.lock().unwrap().push((task_id, text));
        }
    }

    fn new_log() -> Log {
        Arc::new(std::sync::Mutex::new(Vec::new()))
    }

    fn texts(log: &Log, task_id: TaskId) -> Vec<String> {
        log.lock()
            .unwrap()
            .iter()
            .filter(|(id, _)| *id == task_id)
            .map(|(_, text)| text.clone())
            .collect()
    }

    fn expected(range: std::ops::Range<usize>) -> Vec<String> {
        range.map(|i| i.to_string()).collect()
    }

    #[tokio::test]
    async fn one_task_events_arrive_in_order() {
        let log: Log = new_log();
        let sink = QueuedAgentEventSink::new(Arc::new(RecordingSink::new(log.clone())), 16);
        let task: TaskId = TaskId::new();
        for i in 0..1000 {
            sink.handle(task, event(&i.to_string())).await;
        }
        sink.shutdown().await;
        assert_eq!(texts(&log, task), expected(0..1000));
    }

    #[tokio::test]
    async fn interleaved_tasks_keep_per_task_order() {
        let log: Log = new_log();
        let sink = QueuedAgentEventSink::new(Arc::new(RecordingSink::new(log.clone())), 4);
        let tasks: [TaskId; 3] = [TaskId::new(), TaskId::new(), TaskId::new()];
        for i in 0..300 {
            sink.handle(tasks[i % 3], event(&i.to_string())).await;
        }
        sink.shutdown().await;
        for (n, task) in tasks.iter().enumerate() {
            let want: Vec<String> = (0..300)
                .filter(|i| i % 3 == n)
                .map(|i| i.to_string())
                .collect();
            assert_eq!(texts(&log, *task), want);
        }
    }

    #[tokio::test]
    async fn full_queue_backpressures_without_dropping() {
        let log: Log = new_log();
        let mut inner = RecordingSink::new(log.clone());
        inner.delay = Some(Duration::from_millis(1));
        let sink = QueuedAgentEventSink::new(Arc::new(inner), 2);
        let task: TaskId = TaskId::new();
        for i in 0..200 {
            sink.handle(task, event(&i.to_string())).await;
        }
        // Con capacidad 2 el productor solo termina cuando el worker va casi al día.
        let processed: usize = log.lock().unwrap().len();
        assert!(processed >= 200 - 2 - 1, "procesados: {processed}");
        sink.shutdown().await;
        assert_eq!(texts(&log, task), expected(0..200));
    }

    #[tokio::test]
    async fn handle_returns_before_inner_finishes() {
        let log: Log = new_log();
        let gate: Arc<Semaphore> = Arc::new(Semaphore::new(0));
        let mut inner = RecordingSink::new(log.clone());
        inner.gate = Some(gate.clone());
        let sink = QueuedAgentEventSink::new(Arc::new(inner), 8);
        let task: TaskId = TaskId::new();
        let result = tokio::time::timeout(Duration::from_secs(1), async {
            sink.handle(task, event("0")).await;
            sink.handle(task, event("1")).await;
        })
        .await;
        assert!(result.is_ok());
        assert!(log.lock().unwrap().is_empty());
        gate.add_permits(2);
        sink.shutdown().await;
        assert_eq!(texts(&log, task), expected(0..2));
    }

    #[tokio::test]
    async fn shutdown_drains_everything_queued() {
        let log: Log = new_log();
        let mut inner = RecordingSink::new(log.clone());
        inner.delay = Some(Duration::from_millis(1));
        let sink = QueuedAgentEventSink::new(Arc::new(inner), 64);
        let task: TaskId = TaskId::new();
        for i in 0..50 {
            sink.handle(task, event(&i.to_string())).await;
        }
        sink.shutdown().await;
        assert_eq!(texts(&log, task), expected(0..50));
    }

    #[tokio::test]
    async fn panicking_event_does_not_lose_the_rest_of_the_queue() {
        let log: Log = new_log();
        let sink = QueuedAgentEventSink::new(Arc::new(RecordingSink::new(log.clone())), 8);
        let task: TaskId = TaskId::new();
        sink.handle(task, event("before")).await;
        sink.handle(task, event(PANIC_MARKER)).await;
        sink.handle(task, event("after")).await;
        sink.shutdown().await;
        assert_eq!(
            texts(&log, task),
            vec!["before".to_string(), "after".to_string()]
        );
    }

    #[tokio::test]
    async fn dead_worker_falls_back_to_inline_processing() {
        let log: Log = new_log();
        let sink = QueuedAgentEventSink::new(Arc::new(RecordingSink::new(log.clone())), 4);
        let task: TaskId = TaskId::new();
        sink.worker.abort();
        tokio::time::timeout(Duration::from_secs(1), sink.sender.closed())
            .await
            .unwrap();
        sink.handle(task, event("after")).await;
        assert_eq!(texts(&log, task), vec!["after".to_string()]);
        sink.shutdown().await;
    }

    #[tokio::test]
    async fn flush_waits_for_slow_queued_events() {
        let log: Log = new_log();
        let mut inner = RecordingSink::new(log.clone());
        inner.delay = Some(Duration::from_millis(20));
        let sink = QueuedAgentEventSink::new(Arc::new(inner), 64);
        let task: TaskId = TaskId::new();
        for i in 0..10 {
            sink.handle(task, event(&i.to_string())).await;
        }
        sink.flush().await;
        assert_eq!(texts(&log, task), expected(0..10));
        sink.shutdown().await;
    }

    #[tokio::test]
    async fn flush_on_empty_queue_returns() {
        let log: Log = new_log();
        let sink = QueuedAgentEventSink::new(Arc::new(RecordingSink::new(log)), 4);
        let result = tokio::time::timeout(Duration::from_secs(1), sink.flush()).await;
        assert!(result.is_ok());
        sink.shutdown().await;
    }

    #[tokio::test]
    async fn flush_after_worker_died_returns() {
        let log: Log = new_log();
        let sink = QueuedAgentEventSink::new(Arc::new(RecordingSink::new(log)), 4);
        sink.worker.abort();
        tokio::time::timeout(Duration::from_secs(1), sink.sender.closed())
            .await
            .unwrap();
        let result = tokio::time::timeout(Duration::from_secs(1), sink.flush()).await;
        assert!(result.is_ok());
        sink.shutdown().await;
    }

    #[tokio::test]
    async fn events_after_a_flush_are_still_processed() {
        let log: Log = new_log();
        let sink = QueuedAgentEventSink::new(Arc::new(RecordingSink::new(log.clone())), 4);
        let task: TaskId = TaskId::new();
        sink.handle(task, event("0")).await;
        sink.flush().await;
        sink.handle(task, event("1")).await;
        sink.flush().await;
        sink.handle(task, event("2")).await;
        sink.shutdown().await;
        assert_eq!(texts(&log, task), expected(0..3));
    }
}
