use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::{AgentCommand, MessageMode};
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

/// Envía un mensaje del usuario al agente. Si está inactivo es un prompt nuevo;
/// si está trabajando se encola como steer o follow-up según `mode`.
pub struct SendMessage {
    lifecycle: Arc<TaskLifecycle>,
    runtime: Arc<dyn AgentRuntimePort>,
}

impl SendMessage {
    pub fn new(lifecycle: Arc<TaskLifecycle>, runtime: Arc<dyn AgentRuntimePort>) -> Self {
        Self { lifecycle, runtime }
    }

    pub async fn execute(
        &self,
        id: TaskId,
        message: String,
        mode: MessageMode,
    ) -> Result<(), TaskError> {
        let message: String = message.trim().to_string();
        if message.is_empty() {
            return Err(TaskError::EmptyMessage);
        }
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.accepts_messages() {
            return Err(invalid_state("enviar mensaje", task.status));
        }
        let command: AgentCommand = match (task.status, mode) {
            (TaskStatus::Idle, _) => AgentCommand::Prompt { message },
            (_, MessageMode::Steer) => AgentCommand::Steer { message },
            (_, MessageMode::FollowUp) => AgentCommand::FollowUp { message },
        };
        self.runtime.send(id, command).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::MockAgentRuntime;

    fn runtime_expecting(expected: AgentCommand) -> MockAgentRuntime {
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_send()
            .withf(move |_, command: &AgentCommand| *command == expected)
            .times(1)
            .returning(|_, _| Ok(()));
        runtime
    }

    async fn send(
        status: TaskStatus,
        mode: MessageMode,
        runtime: MockAgentRuntime,
    ) -> Result<(), TaskError> {
        let task: Task = test_support::task(status);
        let id: TaskId = task.id;
        SendMessage::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(runtime),
        )
        .execute(id, "  hola ".to_string(), mode)
        .await
    }

    #[tokio::test]
    async fn idle_agent_receives_prompt() {
        let runtime: MockAgentRuntime = runtime_expecting(AgentCommand::Prompt {
            message: "hola".to_string(),
        });
        assert_eq!(
            send(TaskStatus::Idle, MessageMode::Steer, runtime).await,
            Ok(())
        );
    }

    #[tokio::test]
    async fn running_agent_receives_steer_or_follow_up() {
        let steer: MockAgentRuntime = runtime_expecting(AgentCommand::Steer {
            message: "hola".to_string(),
        });
        let follow: MockAgentRuntime = runtime_expecting(AgentCommand::FollowUp {
            message: "hola".to_string(),
        });
        assert_eq!(
            send(TaskStatus::Running, MessageMode::Steer, steer).await,
            Ok(())
        );
        assert_eq!(
            send(TaskStatus::Starting, MessageMode::FollowUp, follow).await,
            Ok(())
        );
    }

    #[tokio::test]
    async fn rejects_messages_when_no_agent_is_live() {
        let result: Result<(), TaskError> = send(
            TaskStatus::AwaitingReview,
            MessageMode::Steer,
            MockAgentRuntime::new(),
        )
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
