use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::task_target::TaskTarget;

#[async_trait::async_trait]
pub trait TaskPathPort: Send + Sync {
    /// Devuelve workspace, título y worktree de la tarea o `TaskNotFound`.
    async fn resolve(&self, task_id: Uuid) -> Result<TaskTarget, TerminalError>;
}
