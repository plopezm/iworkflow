use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::task::TaskId;

/// Receptor de los eventos que emite un agente en ejecución.
/// El runtime lo invoca secuencialmente, en orden de llegada, por cada tarea.
#[async_trait::async_trait]
pub trait AgentEventSink: Send + Sync {
    async fn handle(&self, task_id: TaskId, event: AgentEvent);
}
