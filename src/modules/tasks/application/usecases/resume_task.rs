use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

/// Relanza el agente en el worktree existente continuando su sesión previa.
pub struct ResumeTask {
    lifecycle: Arc<TaskLifecycle>,
    runtime: Arc<dyn AgentRuntimePort>,
    sink: Arc<dyn AgentEventSink>,
}

impl ResumeTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        runtime: Arc<dyn AgentRuntimePort>,
        sink: Arc<dyn AgentEventSink>,
    ) -> Self {
        Self {
            lifecycle,
            runtime,
            sink,
        }
    }

    pub async fn execute(&self, id: TaskId) -> Result<(), TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_resume() {
            return Err(invalid_state("reanudar", task.status));
        }
        self.runtime
            .start(
                id,
                &task.worktree_path,
                true,
                &LaunchOptions::default(),
                self.sink.clone(),
            )
            .await?;
        self.lifecycle.transition(id, TaskStatus::Idle, None).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded, WORKTREE};
    use crate::modules::tasks::mocks::{MockAgentEventSink, MockAgentRuntime};

    #[tokio::test]
    async fn restarts_agent_with_resume_and_goes_idle() {
        let task: Task = test_support::task(TaskStatus::Interrupted);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_start()
            .withf(|_, cwd: &Path, resume: &bool, _, _| cwd == Path::new(WORKTREE) && *resume)
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));

        ResumeTask::new(
            test_support::lifecycle(Some(task), &recorded),
            Arc::new(runtime),
            Arc::new(MockAgentEventSink::new()),
        )
        .execute(id)
        .await
        .expect("reanudada");

        assert_eq!(recorded.statuses(), vec![TaskStatus::Idle]);
    }

    #[tokio::test]
    async fn rejects_running_task() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let result: Result<(), TaskError> = ResumeTask::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(MockAgentRuntime::new()),
            Arc::new(MockAgentEventSink::new()),
        )
        .execute(id)
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
