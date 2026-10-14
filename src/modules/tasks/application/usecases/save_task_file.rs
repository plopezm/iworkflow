use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::ports::outbound::task_file_port::TaskFilePort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Overwrites a file in the worktree of an open task and commits the change on its branch.
///
/// It does not coordinate with a running agent: if the agent is mid-run, its own edits and
/// this one end up in the same worktree and the next commit picks up both.
pub struct SaveTaskFile {
    lifecycle: Arc<TaskLifecycle>,
    files: Arc<dyn TaskFilePort>,
    worktrees: Arc<dyn WorktreePort>,
}

impl SaveTaskFile {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        files: Arc<dyn TaskFilePort>,
        worktrees: Arc<dyn WorktreePort>,
    ) -> Self {
        Self {
            lifecycle,
            files,
            worktrees,
        }
    }

    /// Returns whether a commit was created (`false` if the content was identical).
    pub async fn execute(&self, id: TaskId, rel: &str, content: &str) -> Result<bool, TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.has_worktree() {
            return Err(invalid_state("guardar ficheros", task.status));
        }
        self.files.write(&task.worktree_path, rel, content).await?;
        self.worktrees
            .commit_all(&task.worktree_path, &format!("edit {rel} from diff view"))
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded, WORKTREE};
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::{MockTaskFile, MockWorktree};

    async fn save(
        task: Task,
        files: MockTaskFile,
        worktrees: MockWorktree,
    ) -> Result<bool, TaskError> {
        let id: TaskId = task.id;
        SaveTaskFile::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(files),
            Arc::new(worktrees),
        )
        .execute(id, "src/a.rs", "new")
        .await
    }

    #[tokio::test]
    async fn writes_then_commits_in_the_worktree() {
        let mut files: MockTaskFile = MockTaskFile::new();
        let mut worktrees: MockWorktree = MockWorktree::new();
        let mut seq: mockall::Sequence = mockall::Sequence::new();
        files
            .expect_write()
            .withf(|root, rel: &str, content: &str| {
                root == std::path::Path::new(WORKTREE) && rel == "src/a.rs" && content == "new"
            })
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _, _| Ok(()));
        worktrees
            .expect_commit_all()
            .withf(|path, message: &str| {
                path == std::path::Path::new(WORKTREE) && message == "edit src/a.rs from diff view"
            })
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _| Ok(true));

        let committed: bool = save(test_support::task(TaskStatus::Idle), files, worktrees)
            .await
            .expect("saved");

        assert!(committed);
    }

    #[tokio::test]
    async fn reports_no_commit_when_content_is_identical() {
        let mut files: MockTaskFile = MockTaskFile::new();
        let mut worktrees: MockWorktree = MockWorktree::new();
        files.expect_write().returning(|_, _, _| Ok(()));
        worktrees.expect_commit_all().returning(|_, _| Ok(false));

        let committed: bool = save(
            test_support::task(TaskStatus::AwaitingReview),
            files,
            worktrees,
        )
        .await
        .expect("saved");

        assert!(!committed);
    }

    #[tokio::test]
    async fn rejects_tasks_without_worktree() {
        for status in [
            TaskStatus::Backlog,
            TaskStatus::Merged,
            TaskStatus::Discarded,
        ] {
            let mut files: MockTaskFile = MockTaskFile::new();
            let mut worktrees: MockWorktree = MockWorktree::new();
            files.expect_write().never();
            worktrees.expect_commit_all().never();
            let result: Result<bool, TaskError> =
                save(test_support::task(status), files, worktrees).await;
            assert!(
                matches!(result, Err(TaskError::InvalidState { .. })),
                "{status:?}"
            );
        }
    }

    #[tokio::test]
    async fn write_error_stops_before_commit() {
        let mut files: MockTaskFile = MockTaskFile::new();
        let mut worktrees: MockWorktree = MockWorktree::new();
        files
            .expect_write()
            .returning(|_, _, _| Err(TaskError::FileNotEditable("a".to_string())));
        worktrees.expect_commit_all().never();

        let result: Result<bool, TaskError> =
            save(test_support::task(TaskStatus::Idle), files, worktrees).await;

        assert!(matches!(result, Err(TaskError::FileNotEditable(_))));
    }

    #[tokio::test]
    async fn commit_error_is_propagated() {
        let mut files: MockTaskFile = MockTaskFile::new();
        let mut worktrees: MockWorktree = MockWorktree::new();
        files.expect_write().returning(|_, _, _| Ok(()));
        worktrees
            .expect_commit_all()
            .returning(|_, _| Err(TaskError::Git("boom".to_string())));

        let result: Result<bool, TaskError> =
            save(test_support::task(TaskStatus::Idle), files, worktrees).await;

        assert!(matches!(result, Err(TaskError::Git(_))));
    }
}
