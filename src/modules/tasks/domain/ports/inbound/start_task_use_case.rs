use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};

/// Contrato para arrancar una tarea en `Backlog`; lo consumen la UI y el disparador del grafo.
#[async_trait::async_trait]
pub trait StartTaskUseCase: Send + Sync {
    async fn start(&self, id: TaskId) -> Result<Task, TaskError>;
}
