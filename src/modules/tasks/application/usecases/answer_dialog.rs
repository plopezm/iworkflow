use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::{AgentEvent, DialogAnswer};
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

/// Responde a un diálogo (`select`, `confirm`, `input`, `editor`) abierto por el agente.
pub struct AnswerDialog {
    lifecycle: Arc<TaskLifecycle>,
    runtime: Arc<dyn AgentRuntimePort>,
}

impl AnswerDialog {
    pub fn new(lifecycle: Arc<TaskLifecycle>, runtime: Arc<dyn AgentRuntimePort>) -> Self {
        Self { lifecycle, runtime }
    }

    pub async fn execute(
        &self,
        id: TaskId,
        dialog_id: String,
        answer: DialogAnswer,
    ) -> Result<(), TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.has_live_agent() {
            return Err(invalid_state("responder diálogo", task.status));
        }
        if !self.runtime.is_running(id).await {
            return Err(TaskError::AgentNotRunning);
        }
        if !self.runtime.capabilities(id).dialogs {
            return Err(TaskError::Unsupported(
                "el agente no soporta diálogos interactivos".to_string(),
            ));
        }
        self.runtime
            .send(
                id,
                AgentCommand::AnswerDialog {
                    dialog_id: dialog_id.clone(),
                    answer,
                },
            )
            .await?;
        self.lifecycle
            .record(id, AgentEvent::DialogResolved { dialog_id })
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::domain::models::task_event::TaskEventKind;
    use crate::modules::tasks::mocks::MockAgentRuntime;

    #[tokio::test]
    async fn forwards_answer_and_records_resolution() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| true);
        runtime
            .expect_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime
            .expect_send()
            .withf(|_, c: &AgentCommand| {
                *c == AgentCommand::AnswerDialog {
                    dialog_id: "d1".to_string(),
                    answer: DialogAnswer::Confirmed { confirmed: true },
                }
            })
            .times(1)
            .returning(|_, _| Ok(()));

        AnswerDialog::new(
            test_support::lifecycle(Some(task), &recorded),
            Arc::new(runtime),
        )
        .execute(
            id,
            "d1".to_string(),
            DialogAnswer::Confirmed { confirmed: true },
        )
        .await
        .expect("respondido");

        let events: Vec<TaskEventKind> = recorded.events.lock().expect("lock").clone();
        assert_eq!(
            events,
            vec![TaskEventKind::Agent {
                event: AgentEvent::DialogResolved {
                    dialog_id: "d1".to_string()
                }
            }]
        );
    }

    #[tokio::test]
    async fn rejects_answer_when_runtime_lacks_dialogs() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(|_| true);
        runtime
            .expect_capabilities()
            .returning(|_| AgentCapabilities::minimal());
        runtime.expect_send().never();

        let result: Result<(), TaskError> = AnswerDialog::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(runtime),
        )
        .execute(
            id,
            "d1".to_string(),
            DialogAnswer::Confirmed { confirmed: true },
        )
        .await;

        assert!(matches!(result, Err(TaskError::Unsupported(_))));
    }

    #[tokio::test]
    async fn stopped_task_yields_agent_not_running_before_checking_capabilities() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().times(1).returning(|_| false);
        runtime.expect_capabilities().never();
        runtime.expect_send().never();

        let result: Result<(), TaskError> = AnswerDialog::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(runtime),
        )
        .execute(
            id,
            "d1".to_string(),
            DialogAnswer::Confirmed { confirmed: true },
        )
        .await;

        assert_eq!(result, Err(TaskError::AgentNotRunning));
    }
}
