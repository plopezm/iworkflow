use std::collections::HashMap;
use std::path::Path;
use std::sync::{Arc, Mutex, MutexGuard};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::runtime_id::RuntimeId;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

const REGISTRY_POISONED: &str = "registro de runtimes corrupto";

/// Enruta cada tarea al adaptador del runtime elegido en `LaunchOptions::runtime` (o al
/// predeterminado) y recuerda la elección por tarea para delegar el resto de operaciones.
pub struct AgentRuntimeRouter {
    default: RuntimeId,
    runtimes: HashMap<RuntimeId, Arc<dyn AgentRuntimePort>>,
    chosen: Mutex<HashMap<TaskId, Arc<dyn AgentRuntimePort>>>,
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
        })
    }

    fn resolve(&self, options: &LaunchOptions) -> Result<Arc<dyn AgentRuntimePort>, TaskError> {
        let id: &RuntimeId = options.runtime.as_ref().unwrap_or(&self.default);
        self.runtimes
            .get(id)
            .cloned()
            .ok_or_else(|| TaskError::Unsupported(format!("runtime de agente desconocido: {id}")))
    }

    fn chosen_for(&self, task_id: TaskId) -> Result<Option<Arc<dyn AgentRuntimePort>>, TaskError> {
        let chosen: MutexGuard<'_, HashMap<TaskId, Arc<dyn AgentRuntimePort>>> = self
            .chosen
            .lock()
            .map_err(|_| TaskError::Runtime(REGISTRY_POISONED.to_string()))?;
        Ok(chosen.get(&task_id).cloned())
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
        adapter.start(task_id, cwd, resume, options, sink).await?;
        self.chosen
            .lock()
            .map_err(|_| TaskError::Runtime(REGISTRY_POISONED.to_string()))?
            .insert(task_id, adapter);
        Ok(())
    }

    async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError> {
        match self.chosen_for(task_id)? {
            Some(adapter) => adapter.send(task_id, command).await,
            None => Err(TaskError::AgentNotRunning),
        }
    }

    /// Poda la entrada solo si el adaptador confirma la parada, para poder reintentar si falla.
    async fn stop(&self, task_id: TaskId) -> Result<(), TaskError> {
        let Some(adapter) = self.chosen_for(task_id)? else {
            return Ok(());
        };
        adapter.stop(task_id).await?;
        self.chosen
            .lock()
            .map_err(|_| TaskError::Runtime(REGISTRY_POISONED.to_string()))?
            .remove(&task_id);
        Ok(())
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

    fn capabilities(&self, task_id: TaskId) -> AgentCapabilities {
        match self.chosen_for(task_id) {
            Ok(Some(adapter)) => adapter.capabilities(task_id),
            _ => AgentCapabilities::minimal(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
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
