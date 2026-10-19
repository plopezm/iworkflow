use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::models::worktree::TaskDiff;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Devuelve los cambios de la tarea respecto a la base, también los aún sin commitear mientras
/// existe su worktree; en tareas integradas los regenera desde el rango de commits registrado al integrarlas.
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
        let workspace: WorkspaceRef = match (&task.status, &task.diff_range) {
            (TaskStatus::Merged, Some(range)) => {
                let workspace: WorkspaceRef = self.lifecycle.workspace(&task.workspace_id).await?;
                return self.worktrees.diff_range(&workspace.path, range).await;
            }
            _ if task.status.is_closed() => return Err(invalid_state("ver diff", task.status)),
            _ => self.lifecycle.workspace(&task.workspace_id).await?,
        };
        // Con el agente trabajando los cambios aún no están commiteados: se lee el worktree.
        if self.worktrees.exists(&task.worktree_path).await {
            return self
                .worktrees
                .diff_live(&task.worktree_path, &task.base_branch)
                .await;
        }
        self.worktrees
            .diff(&workspace.path, &task.base_branch, &task.branch)
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::worktree::DiffRange;
    use crate::modules::tasks::mocks::MockWorktree;

    fn sample_range() -> DiffRange {
        DiffRange {
            base_sha: "aaa".to_string(),
            head_sha: "bbb".to_string(),
        }
    }

    fn sample_diff() -> TaskDiff {
        TaskDiff {
            stat: String::new(),
            patch: "diff".to_string(),
            truncated: false,
            commits: Vec::new(),
        }
    }

    async fn diff(task: Task, worktrees: MockWorktree) -> Result<TaskDiff, TaskError> {
        let id: TaskId = task.id;
        GetTaskDiff::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(worktrees),
        )
        .execute(id)
        .await
    }

    #[tokio::test]
    async fn rejects_discarded_tasks() {
        let result: Result<TaskDiff, TaskError> = diff(
            test_support::task(TaskStatus::Discarded),
            MockWorktree::new(),
        )
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }

    #[tokio::test]
    async fn rejects_discarded_tasks_even_with_range() {
        let mut task: Task = test_support::task(TaskStatus::Discarded);
        task.diff_range = Some(sample_range());
        let result: Result<TaskDiff, TaskError> = diff(task, MockWorktree::new()).await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }

    #[tokio::test]
    async fn rejects_merged_tasks_without_range() {
        let result: Result<TaskDiff, TaskError> =
            diff(test_support::task(TaskStatus::Merged), MockWorktree::new()).await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }

    #[tokio::test]
    async fn merged_task_with_range_uses_diff_range() {
        let mut task: Task = test_support::task(TaskStatus::Merged);
        task.diff_range = Some(sample_range());
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_diff_range()
            .withf(|_, range: &DiffRange| *range == sample_range())
            .times(1)
            .returning(|_, _| Ok(sample_diff()));
        worktrees.expect_diff().never();

        let result: TaskDiff = diff(task, worktrees).await.expect("diff");

        assert_eq!(result, sample_diff());
    }

    #[tokio::test]
    async fn open_task_diffs_working_tree_including_uncommitted_changes() {
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_exists().returning(|_| true);
        worktrees
            .expect_diff_live()
            .withf(|path: &Path, base: &str| {
                path == Path::new(test_support::WORKTREE) && base == "main"
            })
            .times(1)
            .returning(|_, _| Ok(sample_diff()));
        worktrees.expect_diff().never();
        worktrees.expect_diff_range().never();

        let result: TaskDiff = diff(test_support::task(TaskStatus::Running), worktrees)
            .await
            .expect("diff");

        assert_eq!(result, sample_diff());
    }

    #[tokio::test]
    async fn open_task_without_worktree_diffs_branch_against_base() {
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_exists().returning(|_| false);
        worktrees.expect_diff_live().never();
        worktrees
            .expect_diff()
            .withf(|_, base: &str, branch: &str| {
                base == "main" && branch == "pi/arregla-el-login-abc"
            })
            .times(1)
            .returning(|_, _, _| Ok(sample_diff()));
        worktrees.expect_diff_range().never();

        let result: TaskDiff = diff(test_support::task(TaskStatus::AwaitingReview), worktrees)
            .await
            .expect("diff");

        assert_eq!(result, sample_diff());
    }
}
