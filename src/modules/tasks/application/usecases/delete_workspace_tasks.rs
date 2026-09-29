use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;

/// Borra en cascada las tareas inactivas de un workspace, al eliminarlo.
pub struct DeleteWorkspaceTasks {
    lifecycle: Arc<TaskLifecycle>,
}

impl DeleteWorkspaceTasks {
    pub fn new(lifecycle: Arc<TaskLifecycle>) -> Self {
        Self { lifecycle }
    }

    pub async fn execute(&self, workspace_id: String) -> Result<(), TaskError> {
        let deleted: Option<Vec<TaskId>> = self
            .lifecycle
            .repository()
            .delete_inactive_by_workspace(workspace_id)
            .await?;
        let ids: Vec<TaskId> = deleted.ok_or(TaskError::WorkspaceHasActiveTasks)?;
        for id in ids {
            self.lifecycle.notify_removed(id).await?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::task::{Task, TaskStatus};
    use crate::modules::tasks::mocks::MockTaskRepository;

    #[tokio::test]
    async fn deletes_inactive_tasks_and_notifies_removal() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_delete_inactive_by_workspace()
            .times(1)
            .returning(move |_| Ok(Some(vec![id])));

        DeleteWorkspaceTasks::new(test_support::lifecycle_with(repository, &recorded))
            .execute("ws-1".to_string())
            .await
            .expect("borrado");

        assert_eq!(recorded.published.lock().expect("lock").len(), 1);
    }

    #[tokio::test]
    async fn rejects_when_any_task_is_active() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_delete_inactive_by_workspace()
            .times(1)
            .returning(|_| Ok(None));

        let result: Result<(), TaskError> =
            DeleteWorkspaceTasks::new(test_support::lifecycle_with(repository, &recorded))
                .execute("ws-1".to_string())
                .await;

        assert_eq!(result, Err(TaskError::WorkspaceHasActiveTasks));
    }
}
