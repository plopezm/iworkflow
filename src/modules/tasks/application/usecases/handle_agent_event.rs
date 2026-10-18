use std::sync::Arc;

use chrono::Utc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::usage_limit::{UsageLimitState, from_epoch_secs};
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
        let task: Option<Task> = if event.is_persistent() {
            Some(self.lifecycle.load(id).await?)
        } else {
            None
        };
        let next: Option<(TaskStatus, Option<String>)> = task
            .as_ref()
            .and_then(|task: &Task| task.status.after_agent_event(&event));
        let usage_limit: Option<UsageLimitState> = task
            .as_ref()
            .and_then(|task: &Task| Self::usage_limit_after(task, &event));
        self.lifecycle.record(id, event).await?;
        if let Some((status, error)) = next {
            self.lifecycle.transition(id, status, error).await?;
        }
        if let Some(state) = usage_limit {
            self.lifecycle
                .repository()
                .set_usage_limit(id, state)
                .await?;
        }
        Ok(())
    }

    /// Nueva planificación de reanudación que provoca el evento, si la hay: un límite de
    /// uso la calcula con backoff y un turno normal completado reinicia los intentos.
    fn usage_limit_after(task: &Task, event: &AgentEvent) -> Option<UsageLimitState> {
        match event {
            AgentEvent::UsageLimitReached { resets_at, .. } if task.status.has_live_agent() => {
                Some(
                    task.usage_limit
                        .after_hit(resets_at.and_then(from_epoch_secs), Utc::now()),
                )
            }
            // Un `RunSettled` tardío del turno que topó con el límite no cuenta como éxito.
            AgentEvent::RunSettled
                if task.usage_limit.attempts > 0 && task.status != TaskStatus::RateLimited =>
            {
                Some(UsageLimitState::default())
            }
            _ => None,
        }
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
    use crate::modules::tasks::mocks::MockTaskRepository;

    async fn handle(status: TaskStatus, event: AgentEvent, recorded: &Recorded) {
        let task: Task = test_support::task(status);
        let id: TaskId = task.id;
        HandleAgentEvent::new(test_support::lifecycle(Some(task), recorded))
            .execute(id, event)
            .await
            .expect("procesado");
    }

    fn repository_recording_usage_limit(
        task: Task,
        recorded: &Recorded,
        saved: Arc<std::sync::Mutex<Vec<UsageLimitState>>>,
    ) -> Arc<TaskLifecycle> {
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), recorded);
        repository
            .expect_set_usage_limit()
            .returning(move |_, state| {
                saved.lock().expect("lock").push(state);
                Ok(())
            });
        test_support::lifecycle_with(repository, recorded)
    }

    #[tokio::test]
    async fn usage_limit_event_parks_task_and_schedules_resume() {
        let recorded: Recorded = Recorded::default();
        let saved: Arc<std::sync::Mutex<Vec<UsageLimitState>>> = Arc::default();
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let resets_at: i64 = Utc::now().timestamp() + 3600;

        HandleAgentEvent::new(repository_recording_usage_limit(
            task,
            &recorded,
            saved.clone(),
        ))
        .execute(
            id,
            AgentEvent::UsageLimitReached {
                resets_at: Some(resets_at),
                message: "límite".to_string(),
            },
        )
        .await
        .expect("procesado");

        assert_eq!(recorded.statuses(), vec![TaskStatus::RateLimited]);
        let states: Vec<UsageLimitState> = saved.lock().expect("lock").clone();
        assert_eq!(states.len(), 1);
        assert_eq!(states[0].attempts, 1);
        assert!(
            states[0]
                .resume_at
                .is_some_and(|at| at.timestamp() > resets_at)
        );
    }

    #[tokio::test]
    async fn settled_run_resets_previous_attempts() {
        let recorded: Recorded = Recorded::default();
        let saved: Arc<std::sync::Mutex<Vec<UsageLimitState>>> = Arc::default();
        let mut task: Task = test_support::task(TaskStatus::Running);
        task.usage_limit = UsageLimitState {
            resume_at: None,
            attempts: 2,
        };
        let id: TaskId = task.id;

        HandleAgentEvent::new(repository_recording_usage_limit(
            task,
            &recorded,
            saved.clone(),
        ))
        .execute(id, AgentEvent::RunSettled)
        .await
        .expect("procesado");

        assert_eq!(
            saved.lock().expect("lock").clone(),
            vec![UsageLimitState::default()]
        );
    }

    #[tokio::test]
    async fn late_settle_while_rate_limited_keeps_attempts() {
        let recorded: Recorded = Recorded::default();
        let saved: Arc<std::sync::Mutex<Vec<UsageLimitState>>> = Arc::default();
        let mut task: Task = test_support::task(TaskStatus::RateLimited);
        task.usage_limit = UsageLimitState {
            resume_at: None,
            attempts: 2,
        };
        let id: TaskId = task.id;

        HandleAgentEvent::new(repository_recording_usage_limit(
            task,
            &recorded,
            saved.clone(),
        ))
        .execute(id, AgentEvent::RunSettled)
        .await
        .expect("procesado");

        assert!(saved.lock().expect("lock").is_empty());
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
