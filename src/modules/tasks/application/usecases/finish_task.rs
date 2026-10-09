use std::sync::Arc;

use crate::modules::tasks::application::services::attachment_cleanup::AttachmentCleanup;
use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Cierra el agente, consolida los cambios pendientes en un commit y deja la
/// tarea lista para revisión manual. Después limpia (mejor esfuerzo) sus imágenes adjuntas.
pub struct FinishTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
    runtime: Arc<dyn AgentRuntimePort>,
    cleanup: Arc<AttachmentCleanup>,
}

impl FinishTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        worktrees: Arc<dyn WorktreePort>,
        runtime: Arc<dyn AgentRuntimePort>,
        cleanup: Arc<AttachmentCleanup>,
    ) -> Self {
        Self {
            lifecycle,
            worktrees,
            runtime,
            cleanup,
        }
    }

    pub async fn execute(&self, id: TaskId) -> Result<(), TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_finish() {
            return Err(invalid_state("finalizar", task.status));
        }
        self.lifecycle
            .transition(id, TaskStatus::Finishing, None)
            .await?;

        if let Err(error) = self.consolidate(&task).await {
            self.lifecycle
                .transition(id, TaskStatus::Failed, Some(error.to_string()))
                .await?;
            return Err(error);
        }
        self.lifecycle
            .transition(id, TaskStatus::AwaitingReview, None)
            .await?;
        self.cleanup.cleanup_task(id).await;
        Ok(())
    }

    async fn consolidate(&self, task: &Task) -> Result<(), TaskError> {
        if self.runtime.is_running(task.id).await {
            self.runtime.stop(task.id).await?;
        }
        let message: String = format!("pi: {}", task.title);
        self.worktrees
            .commit_all(&task.worktree_path, &message)
            .await?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded, WORKTREE};
    use crate::modules::tasks::mocks::{
        MockAgentRuntime, MockAttachmentStorage, MockTaskAttachmentRepository, MockWorktree,
    };

    /// Cleanup whose ports fail the test if they are called.
    fn untouched_cleanup() -> Arc<AttachmentCleanup> {
        Arc::new(AttachmentCleanup::new(
            Arc::new(MockTaskAttachmentRepository::new()),
            Arc::new(MockAttachmentStorage::new()),
        ))
    }

    fn cleanup_expecting(
        repo_result: Result<(), TaskError>,
        storage_result: Result<(), TaskError>,
    ) -> Arc<AttachmentCleanup> {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_delete_by_task()
            .times(1)
            .return_once(move |_| repo_result);
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_delete_task_dir()
            .times(1)
            .return_once(move |_| storage_result);
        Arc::new(AttachmentCleanup::new(
            Arc::new(attachments),
            Arc::new(storage),
        ))
    }

    fn running_runtime() -> MockAgentRuntime {
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| true);
        runtime.expect_stop().times(1).returning(|_| Ok(()));
        runtime
    }

    #[tokio::test]
    async fn stops_agent_commits_and_awaits_review() {
        let task: Task = test_support::task(TaskStatus::Idle);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_commit_all()
            .withf(|path: &Path, message: &str| {
                path == Path::new(WORKTREE) && message == "pi: Arregla el login"
            })
            .times(1)
            .returning(|_, _| Ok(true));

        FinishTask::new(
            test_support::lifecycle(Some(task), &recorded),
            Arc::new(worktrees),
            Arc::new(running_runtime()),
            cleanup_expecting(Ok(()), Ok(())),
        )
        .execute(id)
        .await
        .expect("finalizada");

        assert_eq!(
            recorded.statuses(),
            vec![TaskStatus::Finishing, TaskStatus::AwaitingReview]
        );
    }

    #[tokio::test]
    async fn commit_failure_marks_task_failed() {
        let task: Task = test_support::task(TaskStatus::Interrupted);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| false);
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_commit_all()
            .returning(|_, _| Err(TaskError::Git("index.lock".to_string())));

        let result: Result<(), TaskError> = FinishTask::new(
            test_support::lifecycle(Some(task), &recorded),
            Arc::new(worktrees),
            Arc::new(runtime),
            untouched_cleanup(),
        )
        .execute(id)
        .await;

        assert_eq!(result, Err(TaskError::Git("index.lock".to_string())));
        assert_eq!(
            recorded.statuses(),
            vec![TaskStatus::Finishing, TaskStatus::Failed]
        );
    }

    #[tokio::test]
    async fn rejects_closed_task() {
        let task: Task = test_support::task(TaskStatus::Merged);
        let id: TaskId = task.id;
        let result: Result<(), TaskError> = FinishTask::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(MockWorktree::new()),
            Arc::new(MockAgentRuntime::new()),
            untouched_cleanup(),
        )
        .execute(id)
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }

    #[tokio::test]
    async fn cleanup_failures_do_not_fail_the_finish() {
        let task: Task = test_support::task(TaskStatus::Idle);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_commit_all().returning(|_, _| Ok(true));

        let result: Result<(), TaskError> = FinishTask::new(
            test_support::lifecycle(Some(task), &recorded),
            Arc::new(worktrees),
            Arc::new(running_runtime()),
            cleanup_expecting(
                Err(TaskError::Repository("db".to_string())),
                Err(TaskError::Repository("disk".to_string())),
            ),
        )
        .execute(id)
        .await;

        assert_eq!(result, Ok(()));
        assert_eq!(
            recorded.statuses(),
            vec![TaskStatus::Finishing, TaskStatus::AwaitingReview]
        );
    }
}
