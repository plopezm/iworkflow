use tokio::sync::broadcast;

use crate::modules::tasks::domain::models::task_event::TaskEvent;
use crate::modules::tasks::domain::ports::outbound::task_event_publisher_port::TaskEventPublisherPort;

/// Capacidad del canal; un suscriptor más lento recibe `Lagged` y debe resincronizar.
const CHANNEL_CAPACITY: usize = 4096;

/// Bus en memoria que difunde los eventos de todas las tareas a los suscriptores SSE.
#[derive(Clone)]
pub struct BroadcastTaskEventBus {
    sender: broadcast::Sender<TaskEvent>,
}

impl BroadcastTaskEventBus {
    pub fn new() -> Self {
        let (sender, _) = broadcast::channel::<TaskEvent>(CHANNEL_CAPACITY);
        Self { sender }
    }

    pub fn subscribe(&self) -> broadcast::Receiver<TaskEvent> {
        self.sender.subscribe()
    }
}

impl Default for BroadcastTaskEventBus {
    fn default() -> Self {
        Self::new()
    }
}

impl TaskEventPublisherPort for BroadcastTaskEventBus {
    fn publish(&self, event: TaskEvent) {
        // Sin suscriptores el envío falla y el evento en vivo simplemente se pierde.
        let _receivers: Result<usize, broadcast::error::SendError<TaskEvent>> =
            self.sender.send(event);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::agent_event::AgentEvent;
    use crate::modules::tasks::domain::models::task::TaskId;
    use crate::modules::tasks::domain::models::task_event::TaskEventKind;

    #[tokio::test]
    async fn subscribers_receive_published_events() {
        let bus: BroadcastTaskEventBus = BroadcastTaskEventBus::new();
        let mut receiver: broadcast::Receiver<TaskEvent> = bus.subscribe();
        let event: TaskEvent = TaskEvent::ephemeral(
            TaskId::new(),
            TaskEventKind::Agent {
                event: AgentEvent::RunStarted,
            },
        );

        bus.publish(event.clone());

        assert_eq!(receiver.recv().await.expect("evento"), event);
    }

    #[test]
    fn publishing_without_subscribers_is_harmless() {
        BroadcastTaskEventBus::new().publish(TaskEvent::ephemeral(
            TaskId::new(),
            TaskEventKind::Agent {
                event: AgentEvent::RunSettled,
            },
        ));
    }
}
