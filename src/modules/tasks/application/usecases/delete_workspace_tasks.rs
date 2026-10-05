use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Borra en cascada las tareas inactivas de un workspace, al eliminarlo.
pub struct DeleteWorkspaceTasks {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
}

impl DeleteWorkspaceTasks {
    pub fn new(lifecycle: Arc<TaskLifecycle>, worktrees: Arc<dyn WorktreePort>) -> Self {
        Self {
            lifecycle,
            worktrees,
        }
    }

    pub async fn execute(&self, workspace_id: String) -> Result<(), TaskError> {
        let tasks: Vec<Task> = self
            .lifecycle
            .repository()
            .list(Some(workspace_id.clone()))
            .await?;
        if tasks.iter().any(|task: &Task| !task.status.is_inactive()) {
            return Err(TaskError::WorkspaceHasActiveTasks);
        }
        self.cleanup_closed(&workspace_id, &tasks).await;

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

    /// Limpieza best-effort de worktree y rama de tareas cerradas: un fallo nunca bloquea el borrado.
    async fn cleanup_closed(&self, workspace_id: &str, tasks: &[Task]) {
        let workspace: WorkspaceRef = match self.lifecycle.workspace(workspace_id).await {
            Ok(workspace) => workspace,
            Err(error) => {
                tracing::warn!(%workspace_id, %error, "no se pudo resolver el workspace; se omite la limpieza de worktrees");
                return;
            }
        };
        for task in tasks
            .iter()
            .filter(|task: &&Task| task.status.is_closed() && !task.branch.is_empty())
        {
            let result: Result<(), TaskError> = self
                .worktrees
                .remove(&workspace.path, &task.worktree_path, &task.branch)
                .await;
            if let Err(error) = result {
                tracing::warn!(task_id = %task.id, %error, "no se pudo limpiar el worktree de la tarea");
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::{MockTaskRepository, MockWorktree};

    fn use_case(
        mut repository: MockTaskRepository,
        tasks: Vec<Task>,
        worktrees: MockWorktree,
        recorded: &Recorded,
    ) -> DeleteWorkspaceTasks {
        repository
            .expect_list()
            .returning(move |_| Ok(tasks.clone()));
        DeleteWorkspaceTasks::new(
            test_support::lifecycle_with(repository, recorded),
            Arc::new(worktrees),
        )
    }

    fn repository_deleting(ids: Vec<TaskId>, recorded: &Recorded) -> MockTaskRepository {
        let mut repository: MockTaskRepository = test_support::repository_with(None, recorded);
        repository
            .expect_delete_inactive_by_workspace()
            .times(1)
            .returning(move |_| Ok(Some(ids.clone())));
        repository
    }

    #[tokio::test]
    async fn deletes_inactive_tasks_and_notifies_removal() {
        let mut task: Task = test_support::task(TaskStatus::Backlog);
        task.branch = String::new();
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_remove().never();

        use_case(
            repository_deleting(vec![id], &recorded),
            vec![task],
            worktrees,
            &recorded,
        )
        .execute("ws-1".to_string())
        .await
        .expect("borrado");

        assert_eq!(recorded.published.lock().expect("lock").len(), 1);
    }

    #[tokio::test]
    async fn removes_worktree_of_closed_tasks_with_branch() {
        let merged: Task = test_support::task(TaskStatus::Merged);
        let discarded: Task = test_support::task(TaskStatus::Discarded);
        let mut backlog: Task = test_support::task(TaskStatus::Backlog);
        backlog.branch = String::new();
        let ids: Vec<TaskId> = vec![merged.id, discarded.id, backlog.id];
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_remove()
            .withf(|repo, path, branch: &str| {
                repo == std::path::Path::new(test_support::REPO)
                    && path == std::path::Path::new(test_support::WORKTREE)
                    && branch == "pi/arregla-el-login-abc"
            })
            .times(2)
            .returning(|_, _, _| Ok(()));

        use_case(
            repository_deleting(ids, &recorded),
            vec![merged, discarded, backlog],
            worktrees,
            &recorded,
        )
        .execute("ws-1".to_string())
        .await
        .expect("borrado");

        assert_eq!(recorded.published.lock().expect("lock").len(), 3);
    }

    #[tokio::test]
    async fn cleanup_failure_does_not_block_deletion() {
        let task: Task = test_support::task(TaskStatus::Merged);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Err(TaskError::Git("boom".to_string())));

        use_case(
            repository_deleting(vec![id], &recorded),
            vec![task],
            worktrees,
            &recorded,
        )
        .execute("ws-1".to_string())
        .await
        .expect("borrado");

        assert_eq!(recorded.published.lock().expect("lock").len(), 1);
    }

    #[tokio::test]
    async fn rejects_when_any_task_is_active_without_cleaning() {
        let merged: Task = test_support::task(TaskStatus::Merged);
        let running: Task = test_support::task(TaskStatus::Running);
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository = test_support::repository_with(None, &recorded);
        repository.expect_delete_inactive_by_workspace().never();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_remove().never();

        let result: Result<(), TaskError> =
            use_case(repository, vec![merged, running], worktrees, &recorded)
                .execute("ws-1".to_string())
                .await;

        assert_eq!(result, Err(TaskError::WorkspaceHasActiveTasks));
    }
}
