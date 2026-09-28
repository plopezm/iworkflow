use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::models::worktree::MergeOutcome;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Integra la rama de la tarea en la base tras la revisión manual y elimina el worktree.
/// Ante conflictos deja la tarea en `Conflict` conservando el worktree.
pub struct MergeTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
}

impl MergeTask {
    pub fn new(lifecycle: Arc<TaskLifecycle>, worktrees: Arc<dyn WorktreePort>) -> Self {
        Self {
            lifecycle,
            worktrees,
        }
    }

    pub async fn execute(&self, id: TaskId) -> Result<MergeOutcome, TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_merge() {
            return Err(invalid_state("integrar", task.status));
        }
        let workspace: WorkspaceRef = self.lifecycle.workspace(&task.workspace_id).await?;
        let message: String = format!("Merge tarea pi: {}", task.title);
        let outcome: MergeOutcome = self
            .worktrees
            .merge(&workspace.path, &task.base_branch, &task.branch, &message)
            .await?;

        match &outcome {
            MergeOutcome::Merged => {
                let cleanup_error: Option<String> = self
                    .worktrees
                    .remove(&workspace.path, &task.worktree_path, &task.branch)
                    .await
                    .err()
                    .map(|e: TaskError| format!("integrada, pero no se pudo limpiar: {e}"));
                self.lifecycle
                    .transition(id, TaskStatus::Merged, cleanup_error)
                    .await?;
            }
            MergeOutcome::Conflict { files } => {
                let detail: String = format!("conflictos en: {}", files.join(", "));
                self.lifecycle
                    .transition(id, TaskStatus::Conflict, Some(detail))
                    .await?;
            }
        }
        Ok(outcome)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::MockWorktree;

    async fn merge(
        worktrees: MockWorktree,
        status: TaskStatus,
        recorded: &Recorded,
    ) -> Result<MergeOutcome, TaskError> {
        let task: Task = test_support::task(status);
        let id: TaskId = task.id;
        MergeTask::new(
            test_support::lifecycle(Some(task), recorded),
            Arc::new(worktrees),
        )
        .execute(id)
        .await
    }

    #[tokio::test]
    async fn merges_and_removes_worktree() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_merge()
            .withf(|_, base: &str, branch: &str, _| {
                base == "main" && branch == "pi/arregla-el-login-abc"
            })
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Ok(()));

        let outcome: MergeOutcome = merge(worktrees, TaskStatus::AwaitingReview, &recorded)
            .await
            .expect("merge");

        assert_eq!(outcome, MergeOutcome::Merged);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Merged]);
        assert_eq!(recorded.last_error(), None);
    }

    #[tokio::test]
    async fn conflict_keeps_worktree() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_merge().returning(|_, _, _, _| {
            Ok(MergeOutcome::Conflict {
                files: vec!["src/a.rs".to_string()],
            })
        });
        worktrees.expect_remove().never();

        merge(worktrees, TaskStatus::AwaitingReview, &recorded)
            .await
            .expect("merge");

        assert_eq!(recorded.statuses(), vec![TaskStatus::Conflict]);
        assert_eq!(
            recorded.last_error().as_deref(),
            Some("conflictos en: src/a.rs")
        );
    }

    #[tokio::test]
    async fn blocked_merge_does_not_change_status() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_merge().returning(|_, _, _, _| {
            Err(TaskError::MergeBlocked("cambios sin commitear".to_string()))
        });

        let result: Result<MergeOutcome, TaskError> =
            merge(worktrees, TaskStatus::AwaitingReview, &recorded).await;

        assert!(matches!(result, Err(TaskError::MergeBlocked(_))));
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn rejects_task_not_reviewed() {
        let result: Result<MergeOutcome, TaskError> = merge(
            MockWorktree::new(),
            TaskStatus::Running,
            &Recorded::default(),
        )
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
