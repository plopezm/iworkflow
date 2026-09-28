use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};

#[async_trait::async_trait]
pub trait TaskRepositoryPort: Send + Sync {
    async fn insert(&self, task: &Task) -> Result<(), TaskError>;
    async fn find(&self, id: TaskId) -> Result<Option<Task>, TaskError>;
    /// Más recientes primero. Con `workspace_id` filtra por workspace.
    async fn list(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError>;
    async fn list_by_status(&self, statuses: &[TaskStatus]) -> Result<Vec<Task>, TaskError>;
    async fn update_status(
        &self,
        id: TaskId,
        status: TaskStatus,
        error: Option<String>,
    ) -> Result<(), TaskError>;
    /// Persiste un evento y lo devuelve con su `seq` asignado.
    async fn append_event(&self, id: TaskId, kind: TaskEventKind) -> Result<TaskEvent, TaskError>;
    /// Eventos persistidos con `seq > after_seq`, en orden.
    async fn events(&self, id: TaskId, after_seq: Option<i64>)
    -> Result<Vec<TaskEvent>, TaskError>;
}
