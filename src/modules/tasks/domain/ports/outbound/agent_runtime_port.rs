use std::path::Path;
use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;

/// Gestión de procesos de agente (gentle-shell en modo RPC), uno por tarea.
#[async_trait::async_trait]
pub trait AgentRuntimePort: Send + Sync {
    /// Lanza el agente en `cwd`. Con `resume` continúa la sesión previa de esa carpeta.
    async fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        resume: bool,
        options: &LaunchOptions,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError>;

    async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError>;

    /// Cierra el agente de forma ordenada (o lo mata tras un tiempo) y espera a que termine.
    /// Cuando devuelve, el evento `ProcessExited` ya se ha entregado al sink.
    async fn stop(&self, task_id: TaskId) -> Result<(), TaskError>;

    async fn is_running(&self, task_id: TaskId) -> bool;
}
