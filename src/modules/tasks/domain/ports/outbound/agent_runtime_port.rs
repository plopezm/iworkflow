use std::path::Path;
use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::runtime_id::RuntimeId;
use crate::modules::tasks::domain::models::subagent_task::SubagentTask;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;

/// Control de agentes de código, uno por tarea. El puerto no presupone cómo se
/// comunica cada implementación con su agente.
#[async_trait::async_trait]
pub trait AgentRuntimePort: Send + Sync {
    /// Lanza el agente en `cwd` y empieza a entregar sus eventos al `sink`.
    ///
    /// `resume` significa «continuar la sesión previa de esa carpeta, si el agente lo
    /// soporta»; si no, se arranca una sesión nueva.
    async fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        resume: bool,
        options: &LaunchOptions,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError>;

    /// Entrega una orden al agente de la tarea. Las órdenes que el agente no declara
    /// en [`capabilities`](Self::capabilities) pueden rechazarse con `TaskError::Unsupported`.
    async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError>;

    /// Detiene el agente de forma ordenada (forzándolo si tarda demasiado) y espera a
    /// que termine. Cuando devuelve, el evento `ProcessExited` ya se ha entregado al sink.
    async fn stop(&self, task_id: TaskId) -> Result<(), TaskError>;

    async fn is_running(&self, task_id: TaskId) -> bool;

    /// Último estado conocido de los subagentes de la tarea (se conserva tras terminar el
    /// agente). Vacío si la tarea es desconocida o el runtime no declara `subagents`.
    async fn subagents(&self, task_id: TaskId) -> Vec<SubagentTask>;

    /// Capacidades del agente de la tarea. Para una tarea desconocida devuelve
    /// `AgentCapabilities::minimal()`.
    fn capabilities(&self, task_id: TaskId) -> AgentCapabilities;

    /// Capacidades del runtime que se usaría para un arranque con `runtime` (`None` = el
    /// predeterminado), consultables antes de `start`. Un runtime desconocido devuelve
    /// `AgentCapabilities::minimal()`.
    ///
    /// Los casos de uso de arranque/reanudación la usan para aplicar las capacidades al
    /// lanzar: `ResumeTask` rechaza con `TaskError::Unsupported` si falta `resume`, y ambos
    /// quitan `LaunchOptions::model` si falta `model_selection` y `LaunchOptions::thinking`
    /// si falta `thinking_levels` antes de llamar a `start`.
    fn runtime_capabilities(&self, runtime: Option<&RuntimeId>) -> AgentCapabilities;
}
