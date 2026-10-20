use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_mode::AgentMode;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

/// Cambia el modo de trabajo (agente / plan) de un agente en marcha.
pub struct SetAgentMode {
    lifecycle: Arc<TaskLifecycle>,
    runtime: Arc<dyn AgentRuntimePort>,
}

impl SetAgentMode {
    pub fn new(lifecycle: Arc<TaskLifecycle>, runtime: Arc<dyn AgentRuntimePort>) -> Self {
        Self { lifecycle, runtime }
    }

    pub async fn execute(&self, id: TaskId, mode: AgentMode) -> Result<(), TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.has_live_agent() {
            return Err(invalid_state("cambiar el modo", task.status));
        }
        if !self.runtime.capabilities(id).modes {
            return Err(TaskError::Unsupported(
                "el agente no permite cambiar de modo".to_string(),
            ));
        }
        self.runtime.send(id, AgentCommand::SetMode(mode)).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::MockAgentRuntime;

    fn use_case(task: Task, runtime: MockAgentRuntime) -> SetAgentMode {
        SetAgentMode::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(runtime),
        )
    }

    #[tokio::test]
    async fn sends_the_mode_to_a_live_agent_that_supports_it() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_capabilities()
            .returning(|_| AgentCapabilities {
                modes: true,
                ..AgentCapabilities::minimal()
            });
        runtime
            .expect_send()
            .withf(|_, c: &AgentCommand| *c == AgentCommand::SetMode(AgentMode::Plan))
            .times(1)
            .returning(|_, _| Ok(()));

        assert_eq!(
            use_case(task, runtime).execute(id, AgentMode::Plan).await,
            Ok(())
        );
    }

    #[tokio::test]
    async fn rejects_runtimes_without_modes_and_sends_nothing() {
        let task: Task = test_support::task(TaskStatus::Idle);
        let id: TaskId = task.id;
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_capabilities()
            .returning(|_| AgentCapabilities::minimal());
        runtime.expect_send().never();

        let result: Result<(), TaskError> =
            use_case(task, runtime).execute(id, AgentMode::Agent).await;
        assert!(matches!(result, Err(TaskError::Unsupported(_))));
    }

    #[tokio::test]
    async fn rejects_when_agent_not_live() {
        let task: Task = test_support::task(TaskStatus::Merged);
        let id: TaskId = task.id;
        let result: Result<(), TaskError> = use_case(task, MockAgentRuntime::new())
            .execute(id, AgentMode::Plan)
            .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
