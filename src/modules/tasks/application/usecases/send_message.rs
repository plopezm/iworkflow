use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::{AgentCommand, MessageMode};
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
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
        images: Vec<ImageAttachment>,
        mode: MessageMode,
    ) -> Result<(), TaskError> {
        let message: String = message.trim().to_string();
        let images: Vec<ImageAttachment> = ImageAttachment::validate_all(images)?;
        if message.is_empty() && images.is_empty() {
            return Err(TaskError::EmptyMessage);
        }
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.accepts_messages() {
            return Err(invalid_state("enviar mensaje", task.status));
        }
        let command: AgentCommand = match (task.status, mode) {
            (TaskStatus::Idle, _) => AgentCommand::Prompt { message, images },
            (_, MessageMode::Steer) => AgentCommand::Steer { message, images },
            (_, MessageMode::FollowUp) => AgentCommand::FollowUp { message, images },
        };
        match &command {
            AgentCommand::Steer { .. } if !self.runtime.capabilities(id).steer => {
                return Err(TaskError::Unsupported(
                    "el agente no permite redirigir (steer) mientras trabaja".to_string(),
                ));
            }
            AgentCommand::FollowUp { .. } if !self.runtime.capabilities(id).follow_up => {
                return Err(TaskError::Unsupported(
                    "el agente no permite encolar mensajes (follow-up) mientras trabaja"
                        .to_string(),
                ));
            }
            _ => {}
        }
        self.runtime.send(id, command).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
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

    fn png() -> ImageAttachment {
        ImageAttachment {
            name: "a.png".to_string(),
            mime_type: "image/png".to_string(),
            data: "aGk=".to_string(),
        }
    }

    async fn send(
        status: TaskStatus,
        mode: MessageMode,
        runtime: MockAgentRuntime,
    ) -> Result<(), TaskError> {
        send_with("  hola ", Vec::new(), status, mode, runtime).await
    }

    async fn send_with(
        message: &str,
        images: Vec<ImageAttachment>,
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
        .execute(id, message.to_string(), images, mode)
        .await
    }

    #[tokio::test]
    async fn idle_agent_receives_prompt() {
        let runtime: MockAgentRuntime = runtime_expecting(AgentCommand::Prompt {
            message: "hola".to_string(),
            images: Vec::new(),
        });
        assert_eq!(
            send(TaskStatus::Idle, MessageMode::Steer, runtime).await,
            Ok(())
        );
    }

    #[tokio::test]
    async fn running_agent_receives_steer_or_follow_up() {
        let mut steer: MockAgentRuntime = runtime_expecting(AgentCommand::Steer {
            message: "hola".to_string(),
            images: Vec::new(),
        });
        let mut follow: MockAgentRuntime = runtime_expecting(AgentCommand::FollowUp {
            message: "hola".to_string(),
            images: Vec::new(),
        });
        steer
            .expect_capabilities()
            .returning(|_| AgentCapabilities::full());
        follow
            .expect_capabilities()
            .returning(|_| AgentCapabilities::full());
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

    #[tokio::test]
    async fn forwards_images_and_allows_empty_text_with_images() {
        let runtime: MockAgentRuntime = runtime_expecting(AgentCommand::Prompt {
            message: String::new(),
            images: vec![png()],
        });
        assert_eq!(
            send_with(
                "  ",
                vec![png()],
                TaskStatus::Idle,
                MessageMode::Steer,
                runtime
            )
            .await,
            Ok(())
        );
    }

    #[tokio::test]
    async fn rejects_empty_text_without_images() {
        let result: Result<(), TaskError> = send_with(
            "  ",
            Vec::new(),
            TaskStatus::Idle,
            MessageMode::Steer,
            MockAgentRuntime::new(),
        )
        .await;
        assert_eq!(result, Err(TaskError::EmptyMessage));
    }

    #[tokio::test]
    async fn rejects_invalid_images() {
        let mut bad: ImageAttachment = png();
        bad.mime_type = "application/pdf".to_string();
        let result: Result<(), TaskError> = send_with(
            "hola",
            vec![bad],
            TaskStatus::Idle,
            MessageMode::Steer,
            MockAgentRuntime::new(),
        )
        .await;
        assert!(matches!(result, Err(TaskError::InvalidAttachment(_))));
    }

    #[tokio::test]
    async fn rejects_steer_and_follow_up_when_runtime_lacks_capability() {
        for mode in [MessageMode::Steer, MessageMode::FollowUp] {
            let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
            runtime
                .expect_capabilities()
                .returning(|_| AgentCapabilities::minimal());
            runtime.expect_send().never();
            let result: Result<(), TaskError> = send(TaskStatus::Running, mode, runtime).await;
            assert!(matches!(result, Err(TaskError::Unsupported(_))));
        }
    }

    #[tokio::test]
    async fn idle_prompt_does_not_require_extra_capabilities() {
        let mut runtime: MockAgentRuntime = runtime_expecting(AgentCommand::Prompt {
            message: "hola".to_string(),
            images: Vec::new(),
        });
        runtime.expect_capabilities().never();
        assert_eq!(
            send(TaskStatus::Idle, MessageMode::Steer, runtime).await,
            Ok(())
        );
    }
}
