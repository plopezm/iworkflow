use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Detiene el agente si sigue vivo y elimina worktree y rama sin integrar nada.
pub struct DiscardTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
    runtime: Arc<dyn AgentRuntimePort>,
}

impl DiscardTask {
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
        if !task.status.can_discard() {
            return Err(invalid_state("descartar", task.status));
        }
        if self.runtime.is_running(id).await {
            self.runtime.stop(id).await?;
        }
        let workspace: WorkspaceRef = self.lifecycle.workspace(&task.workspace_id).await?;
        self.worktrees
            .remove(&workspace.path, &task.worktree_path, &task.branch)
            .await?;
        self.lifecycle
            .transition(id, TaskStatus::Discarded, None)
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::{MockAgentRuntime, MockWorktree};

    #[tokio::test]
    async fn stops_live_agent_and_removes_worktree() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| true);
        runtime.expect_stop().times(1).returning(|_| Ok(()));
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Ok(()));

        DiscardTask::new(
            test_support::lifecycle(Some(task), &recorded),
            Arc::new(worktrees),
            Arc::new(runtime),
        )
        .execute(id)
        .await
        .expect("descartada");

        assert_eq!(recorded.statuses(), vec![TaskStatus::Discarded]);
    }

    #[tokio::test]
    async fn rejects_already_merged_task() {
        let task: Task = test_support::task(TaskStatus::Merged);
        let id: TaskId = task.id;
        let result: Result<(), TaskError> = DiscardTask::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(MockWorktree::new()),
            Arc::new(MockAgentRuntime::new()),
        )
        .execute(id)
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
