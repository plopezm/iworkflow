use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::subagent_task::SubagentTask;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;

/// Subagentes de una tarea (último estado conocido); `NotFound` si la tarea no existe.
pub struct GetTaskSubagents {
    lifecycle: Arc<TaskLifecycle>,
    runtime: Arc<dyn AgentRuntimePort>,
}

impl GetTaskSubagents {
    pub fn new(lifecycle: Arc<TaskLifecycle>, runtime: Arc<dyn AgentRuntimePort>) -> Self {
        Self { lifecycle, runtime }
    }

    pub async fn execute(&self, id: TaskId) -> Result<Vec<SubagentTask>, TaskError> {
        self.lifecycle.load(id).await?;
        Ok(self.runtime.subagents(id).await)
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::subagent_task::SubagentStatus;
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::MockAgentRuntime;

    fn subagent() -> SubagentTask {
        SubagentTask {
            id: "t1".to_string(),
            agent: "explore".to_string(),
            label: None,
            prompt: None,
            status: SubagentStatus::Running,
            created_at: None,
            started_at: None,
            ended_at: None,
            last_step: None,
            last_activity_at: None,
            turns: 0,
            tool_calls: 0,
            error: None,
            thread: Vec::new(),
        }
    }

    #[tokio::test]
    async fn missing_task_is_not_found_and_runtime_is_not_asked() {
        let id: TaskId = TaskId::new();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_subagents().never();

        let result: Result<Vec<SubagentTask>, TaskError> = GetTaskSubagents::new(
            test_support::lifecycle(None, &Recorded::default()),
            Arc::new(runtime),
        )
        .execute(id)
        .await;

        assert_eq!(result, Err(TaskError::NotFound(id.to_string())));
    }

    #[tokio::test]
    async fn returns_the_runtime_subagents_of_an_existing_task() {
        let task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_subagents()
            .with(eq(id))
            .times(1)
            .returning(|_| vec![subagent()]);

        let result: Result<Vec<SubagentTask>, TaskError> = GetTaskSubagents::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(runtime),
        )
        .execute(id)
        .await;

        assert_eq!(result, Ok(vec![subagent()]));
    }

    #[tokio::test]
    async fn existing_task_without_subagents_yields_empty() {
        let task = test_support::task(TaskStatus::Merged);
        let id: TaskId = task.id;
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime.expect_subagents().returning(|_| Vec::new());

        let result: Result<Vec<SubagentTask>, TaskError> = GetTaskSubagents::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(runtime),
        )
        .execute(id)
        .await;

        assert_eq!(result, Ok(Vec::new()));
    }
}
