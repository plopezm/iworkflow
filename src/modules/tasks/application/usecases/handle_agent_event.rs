use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;

/// Registra cada evento del agente y aplica las transiciones de estado que provoca.
pub struct HandleAgentEvent {
    lifecycle: Arc<TaskLifecycle>,
}

impl HandleAgentEvent {
    pub fn new(lifecycle: Arc<TaskLifecycle>) -> Self {
        Self { lifecycle }
    }

    pub async fn execute(&self, id: TaskId, event: AgentEvent) -> Result<(), TaskError> {
        let next: Option<(TaskStatus, Option<String>)> = if event.is_persistent() {
            let task: Task = self.lifecycle.load(id).await?;
            task.status.after_agent_event(&event)
        } else {
            None
        };
        self.lifecycle.record(id, event).await?;
        if let Some((status, error)) = next {
            self.lifecycle.transition(id, status, error).await?;
        }
        Ok(())
    }
}

#[async_trait::async_trait]
impl AgentEventSink for HandleAgentEvent {
    async fn handle(&self, task_id: TaskId, event: AgentEvent) {
        if let Err(error) = self.execute(task_id, event).await {
            tracing::warn!(%task_id, %error, "no se pudo registrar el evento del agente");
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::task_event::TaskEventKind;

    async fn handle(status: TaskStatus, event: AgentEvent, recorded: &Recorded) {
        let task: Task = test_support::task(status);
        let id: TaskId = task.id;
        HandleAgentEvent::new(test_support::lifecycle(Some(task), recorded))
            .execute(id, event)
            .await
            .expect("procesado");
    }

    #[tokio::test]
    async fn run_start_moves_task_to_running() {
        let recorded: Recorded = Recorded::default();
        handle(TaskStatus::Starting, AgentEvent::RunStarted, &recorded).await;
        assert_eq!(recorded.statuses(), vec![TaskStatus::Running]);
        assert_eq!(recorded.events.lock().expect("lock").len(), 2);
    }

    #[tokio::test]
    async fn deltas_are_published_but_not_persisted() {
        let recorded: Recorded = Recorded::default();
        handle(
            TaskStatus::Running,
            AgentEvent::TextDelta {
                delta: "ho".to_string(),
            },
            &recorded,
        )
        .await;
        assert!(recorded.events.lock().expect("lock").is_empty());
        assert_eq!(recorded.published.lock().expect("lock").len(), 1);
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn exit_after_finishing_is_recorded_without_status_change() {
        let recorded: Recorded = Recorded::default();
        handle(
            TaskStatus::Finishing,
            AgentEvent::ProcessExited { code: Some(0) },
            &recorded,
        )
        .await;
        assert!(recorded.statuses().is_empty());
        assert_eq!(
            recorded.events.lock().expect("lock").clone(),
            vec![TaskEventKind::Agent {
                event: AgentEvent::ProcessExited { code: Some(0) }
            }]
        );
    }
}
