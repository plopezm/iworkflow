use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::models::task_file_content::TaskFileContent;
use crate::modules::tasks::domain::ports::outbound::task_file_port::TaskFilePort;

/// Reads a file from the worktree of an open task.
pub struct GetTaskFile {
    lifecycle: Arc<TaskLifecycle>,
    files: Arc<dyn TaskFilePort>,
}

impl GetTaskFile {
    pub fn new(lifecycle: Arc<TaskLifecycle>, files: Arc<dyn TaskFilePort>) -> Self {
        Self { lifecycle, files }
    }

    pub async fn execute(&self, id: TaskId, rel: &str) -> Result<TaskFileContent, TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.has_worktree() {
            return Err(invalid_state("leer ficheros", task.status));
        }
        self.files.read(&task.worktree_path, rel).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded, WORKTREE};
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::MockTaskFile;

    async fn get(task: Task, files: MockTaskFile, rel: &str) -> Result<TaskFileContent, TaskError> {
        let id: TaskId = task.id;
        GetTaskFile::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(files),
        )
        .execute(id, rel)
        .await
    }

    #[tokio::test]
    async fn reads_from_the_task_worktree() {
        let mut files: MockTaskFile = MockTaskFile::new();
        files
            .expect_read()
            .withf(|root, rel: &str| root == std::path::Path::new(WORKTREE) && rel == "src/a.rs")
            .times(1)
            .returning(|_, rel: &str| {
                Ok(TaskFileContent {
                    path: rel.to_string(),
                    content: "x".to_string(),
                })
            });

        let result: TaskFileContent = get(
            test_support::task(TaskStatus::AwaitingReview),
            files,
            "src/a.rs",
        )
        .await
        .expect("file");

        assert_eq!(result.content, "x");
    }

    #[tokio::test]
    async fn rejects_tasks_without_worktree() {
        for status in [
            TaskStatus::Backlog,
            TaskStatus::Merged,
            TaskStatus::Discarded,
        ] {
            let mut files: MockTaskFile = MockTaskFile::new();
            files.expect_read().never();
            let result: Result<TaskFileContent, TaskError> =
                get(test_support::task(status), files, "a").await;
            assert!(
                matches!(result, Err(TaskError::InvalidState { .. })),
                "{status:?}"
            );
        }
    }

    #[tokio::test]
    async fn propagates_port_errors() {
        let mut files: MockTaskFile = MockTaskFile::new();
        files
            .expect_read()
            .returning(|_, _| Err(TaskError::InvalidFilePath("..".to_string())));
        let result: Result<TaskFileContent, TaskError> =
            get(test_support::task(TaskStatus::Idle), files, "..").await;
        assert!(matches!(result, Err(TaskError::InvalidFilePath(_))));
    }
}
