use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::models::worktree::TaskDiff;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Devuelve los cambios de la rama de la tarea respecto a la base.
pub struct GetTaskDiff {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
}

impl GetTaskDiff {
    pub fn new(lifecycle: Arc<TaskLifecycle>, worktrees: Arc<dyn WorktreePort>) -> Self {
        Self {
            lifecycle,
            worktrees,
        }
    }

    pub async fn execute(&self, id: TaskId) -> Result<TaskDiff, TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if task.status.is_closed() {
            return Err(invalid_state("ver diff", task.status));
        }
        let workspace: WorkspaceRef = self.lifecycle.workspace(&task.workspace_id).await?;
        self.worktrees
            .diff(&workspace.path, &task.base_branch, &task.branch)
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::MockWorktree;

    #[tokio::test]
    async fn rejects_closed_tasks() {
        let task: Task = test_support::task(TaskStatus::Discarded);
        let id: TaskId = task.id;
        let result: Result<TaskDiff, TaskError> = GetTaskDiff::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(MockWorktree::new()),
        )
        .execute(id)
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
