use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

/// Interrumpe la ejecución en curso del agente (el proceso sigue vivo).
pub struct AbortTask {
    lifecycle: Arc<TaskLifecycle>,
    runtime: Arc<dyn AgentRuntimePort>,
}

impl AbortTask {
    pub fn new(lifecycle: Arc<TaskLifecycle>, runtime: Arc<dyn AgentRuntimePort>) -> Self {
        Self { lifecycle, runtime }
    }

    pub async fn execute(&self, id: TaskId) -> Result<(), TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.has_live_agent() {
            return Err(invalid_state("abortar", task.status));
        }
        self.runtime.send(id, AgentCommand::Abort).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::MockAgentRuntime;

    #[tokio::test]
    async fn sends_abort_to_live_agent() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_send()
            .withf(|_, c: &AgentCommand| *c == AgentCommand::Abort)
            .times(1)
            .returning(|_, _| Ok(()));

        let result: Result<(), TaskError> = AbortTask::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(runtime),
        )
        .execute(id)
        .await;

        assert_eq!(result, Ok(()));
    }

    #[tokio::test]
    async fn rejects_when_agent_not_live() {
        let task: Task = test_support::task(TaskStatus::Merged);
        let id: TaskId = task.id;
        let result: Result<(), TaskError> = AbortTask::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(MockAgentRuntime::new()),
        )
        .execute(id)
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
