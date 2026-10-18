use std::sync::Arc;

use chrono::{DateTime, Utc};

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::application::usecases::resume_task::ResumeTask;
use crate::modules::tasks::application::usecases::send_message::SendMessage;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::MessageMode;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::usage_limit::UsageLimitState;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

const CONTINUE_MESSAGE: &str = "Continúa con la tarea donde la dejaste.";

/// Reanuda las tareas paradas por un límite de uso cuando llega su hora de reintento.
pub struct ResumeRateLimitedTasks {
    lifecycle: Arc<TaskLifecycle>,
    runtime: Arc<dyn AgentRuntimePort>,
    resume_task: Arc<ResumeTask>,
    send_message: Arc<SendMessage>,
}

impl ResumeRateLimitedTasks {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        runtime: Arc<dyn AgentRuntimePort>,
        resume_task: Arc<ResumeTask>,
        send_message: Arc<SendMessage>,
    ) -> Self {
        Self {
            lifecycle,
            runtime,
            resume_task,
            send_message,
        }
    }

    /// Devuelve cuántas tareas se han reanudado. Un fallo en una tarea no detiene al resto.
    pub async fn execute(&self, now: DateTime<Utc>) -> Result<usize, TaskError> {
        let parked: Vec<Task> = self
            .lifecycle
            .repository()
            .list_by_status(&[TaskStatus::RateLimited])
            .await?;
        let mut resumed: usize = 0;
        for task in &parked {
            // El proceso queda inactivo al parar la tarea: se limpia siempre, esté o no a tiempo.
            if self.runtime.is_running(task.id).await
                && let Err(error) = self.runtime.stop(task.id).await
            {
                tracing::warn!(task_id = %task.id, %error, "no se pudo detener el agente inactivo");
            }
            if !task.usage_limit.is_due(now) {
                continue;
            }
            match self.resume(task.id).await {
                Ok(()) => resumed += 1,
                Err(error) => {
                    tracing::warn!(task_id = %task.id, %error, "no se pudo reanudar la tarea tras el límite de uso");
                    self.reschedule(task, now).await;
                }
            }
        }
        Ok(resumed)
    }

    async fn resume(&self, id: TaskId) -> Result<(), TaskError> {
        self.resume_task.execute(id).await?;
        self.send_message
            .execute(
                id,
                CONTINUE_MESSAGE.to_string(),
                Vec::new(),
                MessageMode::FollowUp,
            )
            .await
    }

    /// Retrasa el siguiente intento con el backoff de sondeo.
    async fn reschedule(&self, task: &Task, now: DateTime<Utc>) {
        let next: UsageLimitState = task.usage_limit.after_hit(None, now);
        if let Err(error) = self
            .lifecycle
            .repository()
            .set_usage_limit(task.id, next)
            .await
        {
            tracing::warn!(task_id = %task.id, %error, "no se pudo replanificar la reanudación");
        }
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex;
    use std::sync::atomic::{AtomicUsize, Ordering};

    use chrono::Duration;

    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
    use crate::modules::tasks::domain::models::agent_command::AgentCommand;
    use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
    use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;
    use crate::modules::tasks::mocks::{
        MockAgentEventSink, MockAgentRuntime, MockLaunchSettings, MockTaskRepository, MockWorktree,
    };

    type Scheduled = Arc<Mutex<Vec<UsageLimitState>>>;

    fn parked(resume_in_secs: i64, now: DateTime<Utc>) -> Task {
        let mut task: Task = test_support::task(TaskStatus::RateLimited);
        task.usage_limit = UsageLimitState {
            resume_at: Some(now + Duration::seconds(resume_in_secs)),
            attempts: 1,
        };
        task
    }

    /// Repositorio cuyo `find` ve la tarea parada y, tras reanudarla, inactiva.
    fn lifecycle_for(task: Task, recorded: &Recorded, scheduled: &Scheduled) -> Arc<TaskLifecycle> {
        let mut repository: MockTaskRepository = test_support::repository_without_find(recorded);
        let calls: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
        let listed: Vec<Task> = vec![task.clone()];
        let mut idle: Task = task.clone();
        idle.status = TaskStatus::Idle;
        let parked_task: Task = task;
        repository.expect_find().returning(move |_| {
            let call: usize = calls.fetch_add(1, Ordering::SeqCst);
            Ok(Some(if call == 0 {
                parked_task.clone()
            } else {
                idle.clone()
            }))
        });
        repository
            .expect_list_by_status()
            .returning(move |_| Ok(listed.clone()));
        let sink: Scheduled = scheduled.clone();
        repository
            .expect_set_usage_limit()
            .returning(move |_, state| {
                sink.lock().expect("lock").push(state);
                Ok(())
            });
        test_support::lifecycle_with(repository, recorded)
    }

    fn build(
        lifecycle: Arc<TaskLifecycle>,
        runtime: MockAgentRuntime,
        worktree_exists: bool,
    ) -> ResumeRateLimitedTasks {
        let runtime: Arc<MockAgentRuntime> = Arc::new(runtime);
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_exists()
            .returning(move |_| worktree_exists);
        let worktrees: Arc<dyn WorktreePort> = Arc::new(worktrees);
        let mut settings: MockLaunchSettings = MockLaunchSettings::new();
        settings
            .expect_prepare()
            .returning(|_| Ok(LaunchOptions::default()));
        let resume_task: Arc<ResumeTask> = Arc::new(ResumeTask::new(
            lifecycle.clone(),
            worktrees,
            runtime.clone(),
            Arc::new(settings),
            Arc::new(MockAgentEventSink::new()),
        ));
        let send_message: Arc<SendMessage> =
            Arc::new(SendMessage::new(lifecycle.clone(), runtime.clone()));
        ResumeRateLimitedTasks::new(lifecycle, runtime, resume_task, send_message)
    }

    fn runtime(running: bool) -> MockAgentRuntime {
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_is_running().returning(move |_| running);
        runtime
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime
    }

    #[tokio::test]
    async fn resumes_due_task_and_sends_continuation() {
        let now: DateTime<Utc> = Utc::now();
        let recorded: Recorded = Recorded::default();
        let scheduled: Scheduled = Scheduled::default();
        let mut runtime: MockAgentRuntime = runtime(false);
        runtime
            .expect_start()
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        runtime
            .expect_send()
            .withf(|_, command: &AgentCommand| {
                matches!(command, AgentCommand::Prompt { message, .. } if message == CONTINUE_MESSAGE)
            })
            .times(1)
            .returning(|_, _| Ok(()));
        let lifecycle: Arc<TaskLifecycle> = lifecycle_for(parked(-1, now), &recorded, &scheduled);

        let resumed: usize = build(lifecycle, runtime, true)
            .execute(now)
            .await
            .expect("ok");

        assert_eq!(resumed, 1);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Idle]);
        assert!(scheduled.lock().expect("lock").is_empty());
    }

    #[tokio::test]
    async fn leaves_not_due_task_parked_but_stops_its_idle_process() {
        let now: DateTime<Utc> = Utc::now();
        let recorded: Recorded = Recorded::default();
        let scheduled: Scheduled = Scheduled::default();
        let mut runtime: MockAgentRuntime = runtime(true);
        runtime.expect_stop().times(1).returning(|_| Ok(()));
        let lifecycle: Arc<TaskLifecycle> = lifecycle_for(parked(600, now), &recorded, &scheduled);

        let resumed: usize = build(lifecycle, runtime, true)
            .execute(now)
            .await
            .expect("ok");

        assert_eq!(resumed, 0);
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn failed_resume_reschedules_with_backoff() {
        let now: DateTime<Utc> = Utc::now();
        let recorded: Recorded = Recorded::default();
        let scheduled: Scheduled = Scheduled::default();
        let lifecycle: Arc<TaskLifecycle> = lifecycle_for(parked(-1, now), &recorded, &scheduled);

        let resumed: usize = build(lifecycle, runtime(false), false)
            .execute(now)
            .await
            .expect("ok");

        assert_eq!(resumed, 0);
        let states: Vec<UsageLimitState> = scheduled.lock().expect("lock").clone();
        assert_eq!(states.len(), 1);
        assert_eq!(states[0].attempts, 2);
        assert!(
            states[0]
                .resume_at
                .is_some_and(|at: DateTime<Utc>| at > now)
        );
    }
}
