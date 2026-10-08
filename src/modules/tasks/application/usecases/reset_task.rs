use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Devuelve una tarea a `Backlog`: detiene el agente si sigue vivo, elimina worktree y
/// rama, borra el historial (eventos) y vacía el resto del estado de ejecución.
pub struct ResetTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
    runtime: Arc<dyn AgentRuntimePort>,
}

impl ResetTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        worktrees: Arc<dyn WorktreePort>,
        runtime: Arc<dyn AgentRuntimePort>,
    ) -> Self {
        Self {
            lifecycle,
            worktrees,
            runtime,
        }
    }

    pub async fn execute(&self, id: TaskId) -> Result<(), TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_reset() {
            return Err(invalid_state("restablecer", task.status));
        }
        if self.runtime.is_running(id).await {
            self.runtime.stop(id).await?;
        }
        let workspace: WorkspaceRef = self.lifecycle.workspace(&task.workspace_id).await?;
        // `remove` es idempotente: tolera un worktree o rama ya inexistentes (p. ej. `Discarded`).
        self.worktrees
            .remove(&workspace.path, &task.worktree_path, &task.branch)
            .await?;
        let reset: bool = self.lifecycle.repository().reset_to_backlog(id).await?;
        if !reset {
            return Err(self.lifecycle.conflict(id, "restablecer").await);
        }
        // Los eventos ya se borraron: el evento de estado se emite después para que
        // los clientes abiertos refresquen y el nuevo historial empiece por él.
        self.lifecycle
            .notify_status(id, TaskStatus::Backlog, None)
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::{MockAgentRuntime, MockTaskRepository, MockWorktree};

    #[tokio::test]
    async fn stops_live_agent_removes_worktree_and_resets() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_reset_to_backlog()
            .times(1)
            .returning(|_| Ok(true));
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| true);
        runtime.expect_stop().times(1).returning(|_| Ok(()));
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Ok(()));

        ResetTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            Arc::new(runtime),
        )
        .execute(id)
        .await
        .expect("restablecida");

        assert!(recorded.statuses().is_empty());
        assert_eq!(recorded.events.lock().expect("lock").len(), 1);
    }

    #[tokio::test]
    async fn resets_discarded_task_without_stopping_runtime() {
        let task: Task = test_support::task(TaskStatus::Discarded);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_reset_to_backlog()
            .times(1)
            .returning(|_| Ok(true));
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| false);
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Ok(()));

        ResetTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            Arc::new(runtime),
        )
        .execute(id)
        .await
        .expect("restablecida");
    }

    #[tokio::test]
    async fn rejects_backlog_and_merged_tasks() {
        for status in [
            TaskStatus::Backlog,
            TaskStatus::Merged,
            TaskStatus::Finishing,
        ] {
            let task: Task = test_support::task(status);
            let id: TaskId = task.id;
            let result: Result<(), TaskError> = ResetTask::new(
                test_support::lifecycle(Some(task), &Recorded::default()),
                Arc::new(MockWorktree::new()),
                Arc::new(MockAgentRuntime::new()),
            )
            .execute(id)
            .await;
            assert!(matches!(result, Err(TaskError::InvalidState { .. })));
        }
    }

    #[tokio::test]
    async fn does_not_touch_database_when_worktree_removal_fails() {
        let task: Task = test_support::task(TaskStatus::AwaitingReview);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        // Sin `expect_reset_to_backlog`: cualquier llamada haría fallar el mock.
        let repository: MockTaskRepository = test_support::repository_with(Some(task), &recorded);
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| false);
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Err(TaskError::Git("boom".to_string())));

        let result: Result<(), TaskError> = ResetTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            Arc::new(runtime),
        )
        .execute(id)
        .await;

        assert!(matches!(result, Err(TaskError::Git(_))));
        assert!(recorded.events.lock().expect("lock").is_empty());
    }

    #[tokio::test]
    async fn reports_conflict_when_repository_does_not_apply_reset() {
        let task: Task = test_support::task(TaskStatus::Idle);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_reset_to_backlog()
            .times(1)
            .returning(|_| Ok(false));
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| false);
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_remove().returning(|_, _, _| Ok(()));

        let result: Result<(), TaskError> = ResetTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            Arc::new(runtime),
        )
        .execute(id)
        .await;

        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
        assert!(recorded.events.lock().expect("lock").is_empty());
    }
}
