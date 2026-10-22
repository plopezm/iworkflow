use std::collections::{HashMap, HashSet};
use std::path::Path;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex, MutexGuard};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::runtime_id::RuntimeId;
use crate::modules::tasks::domain::models::subagent_task::SubagentTask;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

const REGISTRY_POISONED: &str = "registro de runtimes corrupto";
const ALREADY_RUNNING: &str = "el agente ya está en ejecución";

/// Adaptador elegido para una tarea y la generación del registro que lo creó.
#[derive(Clone)]
struct Chosen {
    generation: u64,
    adapter: Arc<dyn AgentRuntimePort>,
}

/// Reserva exclusiva de arranque de una tarea; se libera al soltarla (también si `start` falla).
struct StartReservation<'a> {
    starting: &'a Mutex<HashSet<TaskId>>,
    task_id: TaskId,
}

impl Drop for StartReservation<'_> {
    fn drop(&mut self) {
        if let Ok(mut starting) = self.starting.lock() {
            starting.remove(&self.task_id);
        }
    }
}

/// Enruta cada tarea al adaptador del runtime elegido en `LaunchOptions::runtime` (o al
/// predeterminado) y recuerda la elección por tarea para delegar el resto de operaciones.
pub struct AgentRuntimeRouter {
    default: RuntimeId,
    runtimes: HashMap<RuntimeId, Arc<dyn AgentRuntimePort>>,
    chosen: Mutex<HashMap<TaskId, Chosen>>,
    /// Tareas con un `start` en curso: hace atómico el "comprobar y registrar" de `start`.
    starting: Mutex<HashSet<TaskId>>,
    generations: AtomicU64,
}

impl AgentRuntimeRouter {
    /// Falla si `default` no está registrado en `runtimes`.
    pub fn new(
        default: RuntimeId,
        runtimes: HashMap<RuntimeId, Arc<dyn AgentRuntimePort>>,
    ) -> Result<Self, TaskError> {
        if !runtimes.contains_key(&default) {
            return Err(TaskError::Unsupported(format!(
                "el runtime predeterminado '{default}' no está registrado"
            )));
        }
        Ok(Self {
            default,
            runtimes,
            chosen: Mutex::new(HashMap::new()),
            starting: Mutex::new(HashSet::new()),
            generations: AtomicU64::new(0),
        })
    }

    /// Runtime usado cuando `LaunchOptions::runtime` es `None`.
    pub fn default_runtime(&self) -> &RuntimeId {
        &self.default
    }

    fn resolve(&self, options: &LaunchOptions) -> Result<Arc<dyn AgentRuntimePort>, TaskError> {
        let id: &RuntimeId = options.runtime.as_ref().unwrap_or(&self.default);
        self.runtimes
            .get(id)
            .cloned()
            .ok_or_else(|| TaskError::Unsupported(format!("runtime de agente desconocido: {id}")))
    }

    /// Reserva atómicamente el arranque de `task_id`; falla si otro `start` ya está en curso.
    fn reserve_start(&self, task_id: TaskId) -> Result<StartReservation<'_>, TaskError> {
        let inserted: bool = self
            .starting
            .lock()
            .map_err(|_| TaskError::Runtime(REGISTRY_POISONED.to_string()))?
            .insert(task_id);
        if !inserted {
            return Err(TaskError::Runtime(ALREADY_RUNNING.to_string()));
        }
        Ok(StartReservation {
            starting: &self.starting,
            task_id,
        })
    }

    fn chosen_entry(&self, task_id: TaskId) -> Result<Option<Chosen>, TaskError> {
        let chosen: MutexGuard<'_, HashMap<TaskId, Chosen>> = self
            .chosen
            .lock()
            .map_err(|_| TaskError::Runtime(REGISTRY_POISONED.to_string()))?;
        Ok(chosen.get(&task_id).cloned())
    }

    fn chosen_for(&self, task_id: TaskId) -> Result<Option<Arc<dyn AgentRuntimePort>>, TaskError> {
        Ok(self
            .chosen_entry(task_id)?
            .map(|entry: Chosen| entry.adapter))
    }
}

#[async_trait::async_trait]
impl AgentRuntimePort for AgentRuntimeRouter {
    async fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        resume: bool,
        options: &LaunchOptions,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError> {
        let adapter: Arc<dyn AgentRuntimePort> = self.resolve(options)?;
        // Mantenida hasta registrar la elección: dos `start` concurrentes de la misma tarea no
        // pueden pasar ambos la comprobación de "ya en ejecución" y lanzar dos agentes.
        let _reservation: StartReservation<'_> = self.reserve_start(task_id)?;
        if let Some(recorded) = self.chosen_for(task_id)?
            && recorded.is_running(task_id).await
        {
            return Err(TaskError::Runtime(ALREADY_RUNNING.to_string()));
        }
        adapter.start(task_id, cwd, resume, options, sink).await?;
        let generation: u64 = self.generations.fetch_add(1, Ordering::Relaxed);
        self.chosen
            .lock()
            .map_err(|_| TaskError::Runtime(REGISTRY_POISONED.to_string()))?
            .insert(
                task_id,
                Chosen {
                    generation,
                    adapter,
                },
            );
        Ok(())
    }

    async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError> {
        match self.chosen_for(task_id)? {
            Some(adapter) => adapter.send(task_id, command).await,
            None => Err(TaskError::AgentNotRunning),
        }
    }

    /// Poda la entrada solo si el adaptador confirma la parada, para poder reintentar si falla,
    /// y solo si sigue siendo la misma generación: un `start` concurrente que la haya recreado
    /// mientras se esperaba al adaptador no se borra.
    async fn stop(&self, task_id: TaskId) -> Result<(), TaskError> {
        let Some(entry) = self.chosen_entry(task_id)? else {
            return Ok(());
        };
        entry.adapter.stop(task_id).await?;
        let mut chosen: MutexGuard<'_, HashMap<TaskId, Chosen>> = self
            .chosen
            .lock()
            .map_err(|_| TaskError::Runtime(REGISTRY_POISONED.to_string()))?;
        if chosen
            .get(&task_id)
            .is_some_and(|current: &Chosen| current.generation == entry.generation)
        {
            chosen.remove(&task_id);
        }
        Ok(())
    }

    /// Cada adaptador conoce sus procesos: se delega en todos, no solo en los elegidos, y se
    /// vacía el registro de elecciones porque ya no queda ningún agente.
    async fn stop_all(&self) {
        futures::future::join_all(
            self.runtimes
                .values()
                .map(|adapter: &Arc<dyn AgentRuntimePort>| adapter.stop_all()),
        )
        .await;
        if let Ok(mut chosen) = self.chosen.lock() {
            chosen.clear();
        }
    }

    /// No poda al devolver `false`: un agente terminado sigue consultando sus capacidades, y
    /// podar aquí podría borrar la entrada de un `start` concurrente (p. ej. al reanudar).
    /// El mapa queda acotado porque `stop` (finalizar/descartar/reiniciar) lo limpia.
    async fn is_running(&self, task_id: TaskId) -> bool {
        match self.chosen_for(task_id) {
            Ok(Some(adapter)) => adapter.is_running(task_id).await,
            _ => false,
        }
    }

    async fn subagents(&self, task_id: TaskId) -> Vec<SubagentTask> {
        match self.chosen_for(task_id) {
            Ok(Some(adapter)) => adapter.subagents(task_id).await,
            _ => Vec::new(),
        }
    }

    fn capabilities(&self, task_id: TaskId) -> AgentCapabilities {
        match self.chosen_for(task_id) {
            Ok(Some(adapter)) => adapter.capabilities(task_id),
            _ => AgentCapabilities::minimal(),
        }
    }

    fn runtime_capabilities(&self, runtime: Option<&RuntimeId>) -> AgentCapabilities {
        let id: &RuntimeId = runtime.unwrap_or(&self.default);
        match self.runtimes.get(id) {
            Some(adapter) => adapter.runtime_capabilities(Some(id)),
            None => AgentCapabilities::minimal(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::subagent_task::SubagentStatus;
    use crate::modules::tasks::mocks::{MockAgentEventSink, MockAgentRuntime};

    fn id(raw: &str) -> RuntimeId {
        RuntimeId::parse(raw).expect("runtime válido")
    }

    fn started_mock() -> MockAgentRuntime {
        let mut mock: MockAgentRuntime = MockAgentRuntime::new();
        mock.expect_start()
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        mock
    }

    fn untouched_mock() -> MockAgentRuntime {
        let mut mock: MockAgentRuntime = MockAgentRuntime::new();
        mock.expect_start().never();
        mock.expect_send().never();
        mock.expect_stop().never();
        mock.expect_is_running().never();
        mock
    }

    fn router(
        default: MockAgentRuntime,
        other: MockAgentRuntime,
    ) -> Result<AgentRuntimeRouter, TaskError> {
        let mut runtimes: HashMap<RuntimeId, Arc<dyn AgentRuntimePort>> = HashMap::new();
        runtimes.insert(id("gentle-shell"), Arc::new(default));
        runtimes.insert(id("pi"), Arc::new(other));
        AgentRuntimeRouter::new(id("gentle-shell"), runtimes)
    }

    async fn start_with(
        router: &AgentRuntimeRouter,
        task_id: TaskId,
        runtime: Option<&str>,
    ) -> Result<(), TaskError> {
        let options: LaunchOptions = LaunchOptions {
            runtime: runtime.map(id),
            ..LaunchOptions::default()
        };
        let sink: Arc<dyn AgentEventSink> = Arc::new(MockAgentEventSink::new());
        router
            .start(task_id, Path::new("/tmp"), false, &options, sink)
            .await
    }

    #[tokio::test]
    async fn no_runtime_routes_to_the_default() {
        let router: AgentRuntimeRouter = router(started_mock(), untouched_mock()).expect("router");
        start_with(&router, TaskId::new(), None)
            .await
            .expect("start");
    }

    #[tokio::test]
    async fn explicit_runtime_routes_to_that_adapter() {
        let router: AgentRuntimeRouter = router(untouched_mock(), started_mock()).expect("router");
        start_with(&router, TaskId::new(), Some("pi"))
            .await
            .expect("start");
    }

    #[tokio::test]
    async fn unknown_runtime_is_unsupported_and_launches_nothing() {
        let router: AgentRuntimeRouter =
            router(untouched_mock(), untouched_mock()).expect("router");
        let task_id: TaskId = TaskId::new();

        let result: Result<(), TaskError> = start_with(&router, task_id, Some("codex")).await;

        assert!(matches!(
            result,
            Err(TaskError::Unsupported(message)) if message.contains("codex")
        ));
        assert_eq!(
            router.send(task_id, AgentCommand::Abort).await,
            Err(TaskError::AgentNotRunning)
        );
    }

    #[tokio::test]
    async fn failed_start_is_not_recorded() {
        let mut failing: MockAgentRuntime = MockAgentRuntime::new();
        failing
            .expect_start()
            .times(1)
            .returning(|_, _, _, _, _| Err(TaskError::Runtime("boom".to_string())));
        let router: AgentRuntimeRouter = router(failing, untouched_mock()).expect("router");
        let task_id: TaskId = TaskId::new();

        let result: Result<(), TaskError> = start_with(&router, task_id, None).await;

        assert_eq!(result, Err(TaskError::Runtime("boom".to_string())));
        assert!(!router.is_running(task_id).await);
        assert_eq!(
            router.send(task_id, AgentCommand::Abort).await,
            Err(TaskError::AgentNotRunning)
        );
    }

    #[tokio::test]
    async fn operations_delegate_to_the_recorded_adapter() {
        let mut pi: MockAgentRuntime = started_mock();
        pi.expect_send().times(1).returning(|_, _| Ok(()));
        pi.expect_is_running().times(1).returning(|_| true);
        pi.expect_capabilities()
            .times(1)
            .returning(|_| AgentCapabilities::full());
        let router: AgentRuntimeRouter = router(untouched_mock(), pi).expect("router");
        let task_id: TaskId = TaskId::new();
        start_with(&router, task_id, Some("pi"))
            .await
            .expect("start");

        assert_eq!(router.send(task_id, AgentCommand::Abort).await, Ok(()));
        assert!(router.is_running(task_id).await);
        assert_eq!(router.capabilities(task_id), AgentCapabilities::full());
    }

    #[tokio::test]
    async fn subagents_delegate_to_the_recorded_adapter() {
        let mut pi: MockAgentRuntime = started_mock();
        pi.expect_subagents().times(1).returning(|_| {
            vec![SubagentTask {
                id: "t1".to_string(),
                agent: "explore".to_string(),
                label: None,
                prompt: None,
                status: SubagentStatus::Running,
                created_at: None,
                started_at: None,
                ended_at: None,
                last_step: None,
                last_activity_at: None,
                turns: 0,
                tool_calls: 0,
                error: None,
                thread: Vec::new(),
            }]
        });
        let router: AgentRuntimeRouter = router(untouched_mock(), pi).expect("router");
        let task_id: TaskId = TaskId::new();
        start_with(&router, task_id, Some("pi"))
            .await
            .expect("start");

        let subagents: Vec<SubagentTask> = router.subagents(task_id).await;

        assert_eq!(subagents.len(), 1);
        assert_eq!(subagents[0].id, "t1");
    }

    #[tokio::test]
    async fn subagents_of_an_unknown_task_are_empty() {
        let router: AgentRuntimeRouter =
            router(untouched_mock(), untouched_mock()).expect("router");

        assert!(router.subagents(TaskId::new()).await.is_empty());
    }

    #[tokio::test]
    async fn stop_all_reaches_every_registered_runtime() {
        let mut default: MockAgentRuntime = untouched_mock();
        default.expect_stop_all().times(1).returning(|| ());
        let mut pi: MockAgentRuntime = started_mock();
        pi.expect_stop_all().times(1).returning(|| ());
        let router: AgentRuntimeRouter = router(default, pi).expect("router");
        let task_id: TaskId = TaskId::new();
        start_with(&router, task_id, Some("pi"))
            .await
            .expect("start");

        router.stop_all().await;

        assert_eq!(
            router.send(task_id, AgentCommand::Abort).await,
            Err(TaskError::AgentNotRunning)
        );
    }

    #[tokio::test]
    async fn stop_delegates_and_prunes_the_entry() {
        let mut pi: MockAgentRuntime = started_mock();
        pi.expect_stop().times(1).returning(|_| Ok(()));
        let router: AgentRuntimeRouter = router(untouched_mock(), pi).expect("router");
        let task_id: TaskId = TaskId::new();
        start_with(&router, task_id, Some("pi"))
            .await
            .expect("start");

        assert_eq!(router.stop(task_id).await, Ok(()));

        assert_eq!(
            router.send(task_id, AgentCommand::Abort).await,
            Err(TaskError::AgentNotRunning)
        );
        assert_eq!(router.capabilities(task_id), AgentCapabilities::minimal());
        // Un segundo stop ya no llega al adaptador (stop.times(1)).
        assert_eq!(router.stop(task_id).await, Ok(()));
    }

    #[tokio::test]
    async fn failed_stop_keeps_the_entry_for_retry() {
        let mut pi: MockAgentRuntime = started_mock();
        pi.expect_stop()
            .times(1)
            .returning(|_| Err(TaskError::Runtime("no para".to_string())));
        pi.expect_is_running().times(1).returning(|_| true);
        let router: AgentRuntimeRouter = router(untouched_mock(), pi).expect("router");
        let task_id: TaskId = TaskId::new();
        start_with(&router, task_id, Some("pi"))
            .await
            .expect("start");

        assert!(router.stop(task_id).await.is_err());

        assert!(router.is_running(task_id).await);
    }

    #[tokio::test]
    async fn unknown_task_uses_safe_fallbacks() {
        let router: AgentRuntimeRouter =
            router(untouched_mock(), untouched_mock()).expect("router");
        let task_id: TaskId = TaskId::new();

        assert!(!router.is_running(task_id).await);
        assert_eq!(
            router.send(task_id, AgentCommand::Abort).await,
            Err(TaskError::AgentNotRunning)
        );
        assert_eq!(router.stop(task_id).await, Ok(()));
        assert_eq!(router.capabilities(task_id), AgentCapabilities::minimal());
    }

    #[tokio::test]
    async fn start_is_rejected_while_the_recorded_adapter_still_runs() {
        let mut shell: MockAgentRuntime = started_mock();
        shell.expect_is_running().times(1).returning(|_| true);
        let router: AgentRuntimeRouter = router(shell, untouched_mock()).expect("router");
        let task_id: TaskId = TaskId::new();
        start_with(&router, task_id, None).await.expect("start");

        // `pi` es un mock sin `start` esperado: llamarlo haría fallar el test.
        let result: Result<(), TaskError> = start_with(&router, task_id, Some("pi")).await;

        assert_eq!(
            result,
            Err(TaskError::Runtime(
                "el agente ya está en ejecución".to_string()
            ))
        );
    }

    #[tokio::test]
    async fn start_is_allowed_again_once_the_recorded_adapter_stopped() {
        let mut shell: MockAgentRuntime = started_mock();
        shell.expect_is_running().times(1).returning(|_| false);
        let mut pi: MockAgentRuntime = started_mock();
        pi.expect_capabilities()
            .returning(|_| AgentCapabilities::full());
        let router: AgentRuntimeRouter = router(shell, pi).expect("router");
        let task_id: TaskId = TaskId::new();
        start_with(&router, task_id, None).await.expect("start");

        start_with(&router, task_id, Some("pi"))
            .await
            .expect("reinicio");

        assert_eq!(router.capabilities(task_id), AgentCapabilities::full());
    }

    /// Runtime cuyo `stop` relanza la misma tarea a través del router, simulando un `start`
    /// concurrente que se cuela mientras `stop` espera al adaptador.
    struct RestartingOnStop {
        router: Arc<std::sync::OnceLock<Arc<AgentRuntimeRouter>>>,
        task_id: TaskId,
    }

    #[async_trait::async_trait]
    impl AgentRuntimePort for RestartingOnStop {
        async fn start(
            &self,
            _: TaskId,
            _: &Path,
            _: bool,
            _: &LaunchOptions,
            _: Arc<dyn AgentEventSink>,
        ) -> Result<(), TaskError> {
            Ok(())
        }

        async fn send(&self, _: TaskId, _: AgentCommand) -> Result<(), TaskError> {
            Ok(())
        }

        async fn stop(&self, _: TaskId) -> Result<(), TaskError> {
            let router: &Arc<AgentRuntimeRouter> = self.router.get().expect("router listo");
            start_with(router, self.task_id, Some("pi")).await
        }

        async fn is_running(&self, _: TaskId) -> bool {
            false
        }

        async fn subagents(&self, _: TaskId) -> Vec<SubagentTask> {
            Vec::new()
        }

        fn capabilities(&self, _: TaskId) -> AgentCapabilities {
            AgentCapabilities::minimal()
        }

        fn runtime_capabilities(&self, _: Option<&RuntimeId>) -> AgentCapabilities {
            AgentCapabilities::minimal()
        }
    }

    #[tokio::test]
    async fn stop_does_not_prune_an_entry_recreated_by_a_concurrent_start() {
        let task_id: TaskId = TaskId::new();
        let cell: Arc<std::sync::OnceLock<Arc<AgentRuntimeRouter>>> =
            Arc::new(std::sync::OnceLock::new());
        let mut pi: MockAgentRuntime = started_mock();
        pi.expect_capabilities()
            .returning(|_| AgentCapabilities::full());
        let mut runtimes: HashMap<RuntimeId, Arc<dyn AgentRuntimePort>> = HashMap::new();
        runtimes.insert(
            id("gentle-shell"),
            Arc::new(RestartingOnStop {
                router: cell.clone(),
                task_id,
            }),
        );
        runtimes.insert(id("pi"), Arc::new(pi));
        let router: Arc<AgentRuntimeRouter> =
            Arc::new(AgentRuntimeRouter::new(id("gentle-shell"), runtimes).expect("router"));
        assert!(cell.set(router.clone()).is_ok());
        start_with(&router, task_id, None).await.expect("start");

        router.stop(task_id).await.expect("stop");

        assert_eq!(router.capabilities(task_id), AgentCapabilities::full());
    }

    /// Runtime cuyo `start` queda bloqueado hasta que se libera, contando los lanzamientos.
    struct BlockingStart {
        started: Arc<std::sync::atomic::AtomicUsize>,
        release: Arc<tokio::sync::Notify>,
    }

    #[async_trait::async_trait]
    impl AgentRuntimePort for BlockingStart {
        async fn start(
            &self,
            _: TaskId,
            _: &Path,
            _: bool,
            _: &LaunchOptions,
            _: Arc<dyn AgentEventSink>,
        ) -> Result<(), TaskError> {
            self.started.fetch_add(1, Ordering::SeqCst);
            self.release.notified().await;
            Ok(())
        }

        async fn send(&self, _: TaskId, _: AgentCommand) -> Result<(), TaskError> {
            Ok(())
        }

        async fn stop(&self, _: TaskId) -> Result<(), TaskError> {
            Ok(())
        }

        async fn is_running(&self, _: TaskId) -> bool {
            false
        }

        async fn subagents(&self, _: TaskId) -> Vec<SubagentTask> {
            Vec::new()
        }

        fn capabilities(&self, _: TaskId) -> AgentCapabilities {
            AgentCapabilities::minimal()
        }

        fn runtime_capabilities(&self, _: Option<&RuntimeId>) -> AgentCapabilities {
            AgentCapabilities::minimal()
        }
    }

    #[tokio::test]
    async fn concurrent_starts_of_the_same_task_launch_a_single_agent() {
        let started: Arc<std::sync::atomic::AtomicUsize> =
            Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let release: Arc<tokio::sync::Notify> = Arc::new(tokio::sync::Notify::new());
        let mut runtimes: HashMap<RuntimeId, Arc<dyn AgentRuntimePort>> = HashMap::new();
        runtimes.insert(
            id("gentle-shell"),
            Arc::new(BlockingStart {
                started: started.clone(),
                release: release.clone(),
            }),
        );
        let router: AgentRuntimeRouter =
            AgentRuntimeRouter::new(id("gentle-shell"), runtimes).expect("router");
        let task_id: TaskId = TaskId::new();

        let (first, second): (Result<(), TaskError>, Result<(), TaskError>) =
            tokio::join!(start_with(&router, task_id, None), async {
                // Cede el turno para que el primer `start` quede esperando dentro del adaptador.
                tokio::task::yield_now().await;
                let result: Result<(), TaskError> = start_with(&router, task_id, None).await;
                release.notify_one();
                result
            });

        assert_eq!(first, Ok(()));
        assert_eq!(second, Err(TaskError::Runtime(ALREADY_RUNNING.to_string())));
        assert_eq!(started.load(Ordering::SeqCst), 1);
        // La reserva se libera al terminar: un nuevo `start` vuelve a ser posible.
        release.notify_one();
        assert_eq!(start_with(&router, task_id, None).await, Ok(()));
    }

    #[test]
    fn default_must_be_registered() {
        let mut runtimes: HashMap<RuntimeId, Arc<dyn AgentRuntimePort>> = HashMap::new();
        runtimes.insert(id("pi"), Arc::new(MockAgentRuntime::new()));

        let result: Result<AgentRuntimeRouter, TaskError> =
            AgentRuntimeRouter::new(id("gentle-shell"), runtimes);

        assert!(matches!(
            result,
            Err(TaskError::Unsupported(message)) if message.contains("gentle-shell")
        ));
    }
}
