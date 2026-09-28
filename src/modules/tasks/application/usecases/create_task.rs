use std::path::PathBuf;
use std::sync::Arc;

use chrono::Utc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::task::{
    NewTask, Task, TaskId, TaskStatus, branch_name, slugify, worktree_dir_name,
};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Crea el worktree de la tarea, lanza el agente y le envía el prompt inicial.
/// Si el agente no arranca, la tarea queda en `Failed` (el worktree se conserva).
pub struct CreateTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
    runtime: Arc<dyn AgentRuntimePort>,
    sink: Arc<dyn AgentEventSink>,
}

impl CreateTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        worktrees: Arc<dyn WorktreePort>,
        runtime: Arc<dyn AgentRuntimePort>,
        sink: Arc<dyn AgentEventSink>,
    ) -> Self {
        Self {
            lifecycle,
            worktrees,
            runtime,
            sink,
        }
    }

    pub async fn execute(&self, input: NewTask) -> Result<Task, TaskError> {
        let input: NewTask = input.validated()?;
        let workspace: WorkspaceRef = self.lifecycle.workspace(&input.workspace_id).await?;

        let id: TaskId = TaskId::new();
        let branch: String = branch_name(&id, &input.title);
        let worktree_path: PathBuf = self.worktrees.worktree_path(
            &slugify(&workspace.name),
            &worktree_dir_name(&id, &input.title),
        );
        self.worktrees
            .create(
                &workspace.path,
                &workspace.base_branch,
                &branch,
                &worktree_path,
            )
            .await?;

        let now: chrono::DateTime<Utc> = Utc::now();
        let mut task: Task = Task {
            id,
            workspace_id: workspace.id,
            title: input.title,
            prompt: input.prompt,
            branch,
            base_branch: workspace.base_branch,
            worktree_path,
            status: TaskStatus::Starting,
            error: None,
            created_at: now,
            updated_at: now,
        };
        self.lifecycle.repository().insert(&task).await?;
        self.lifecycle
            .transition(id, TaskStatus::Starting, None)
            .await?;

        if let Err(error) = self.launch(&task).await {
            let message: String = error.to_string();
            self.lifecycle
                .transition(id, TaskStatus::Failed, Some(message.clone()))
                .await?;
            task.status = TaskStatus::Failed;
            task.error = Some(message);
        }
        Ok(task)
    }

    async fn launch(&self, task: &Task) -> Result<(), TaskError> {
        self.runtime
            .start(task.id, &task.worktree_path, false, self.sink.clone())
            .await?;
        self.runtime
            .send(
                task.id,
                AgentCommand::Prompt {
                    message: task.prompt.clone(),
                },
            )
            .await
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;
    use crate::modules::tasks::application::test_support::{self, REPO, Recorded};
    use crate::modules::tasks::mocks::{
        MockAgentEventSink, MockAgentRuntime, MockTaskEventPublisher, MockTaskRepository,
        MockWorkspaceLookup, MockWorktree,
    };

    fn input() -> NewTask {
        NewTask {
            workspace_id: "ws-1".to_string(),
            title: "Arregla login".to_string(),
            prompt: "Arregla el login roto".to_string(),
        }
    }

    fn worktrees() -> MockWorktree {
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_worktree_path()
            .returning(|ws, dir| PathBuf::from(format!("/wt/{ws}/{dir}")));
        worktrees
            .expect_create()
            .withf(|repo: &Path, base: &str, branch: &str, _| {
                repo == Path::new(REPO) && base == "main" && branch.starts_with("pi/arregla-login-")
            })
            .times(1)
            .returning(|_, _, _, _| Ok(()));
        worktrees
    }

    fn lifecycle_accepting_insert(recorded: &Recorded) -> Arc<TaskLifecycle> {
        let mut repository: MockTaskRepository = test_support::repository_with(None, recorded);
        repository.expect_insert().times(1).returning(|_| Ok(()));
        Arc::new(TaskLifecycle::new(
            Arc::new(repository),
            Arc::new(test_support::publisher(recorded)),
            Arc::new(test_support::workspaces()),
        ))
    }

    #[tokio::test]
    async fn creates_worktree_starts_agent_and_sends_prompt() {
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_start()
            .withf(|_, cwd: &Path, resume: &bool, _| cwd.starts_with("/wt/demo") && !resume)
            .times(1)
            .returning(|_, _, _, _| Ok(()));
        runtime
            .expect_send()
            .withf(|_, command: &AgentCommand| {
                *command
                    == AgentCommand::Prompt {
                        message: "Arregla el login roto".to_string(),
                    }
            })
            .times(1)
            .returning(|_, _| Ok(()));
        let use_case: CreateTask = CreateTask::new(
            lifecycle_accepting_insert(&recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            Arc::new(MockAgentEventSink::new()),
        );

        let task: Task = use_case.execute(input()).await.expect("creada");

        assert_eq!(task.status, TaskStatus::Starting);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Starting]);
    }

    #[tokio::test]
    async fn marks_task_failed_when_agent_cannot_start() {
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_start()
            .returning(|_, _, _, _| Err(TaskError::Runtime("no existe gentle-shell".to_string())));
        let use_case: CreateTask = CreateTask::new(
            lifecycle_accepting_insert(&recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            Arc::new(MockAgentEventSink::new()),
        );

        let task: Task = use_case.execute(input()).await.expect("creada");

        assert_eq!(task.status, TaskStatus::Failed);
        assert_eq!(
            recorded.statuses(),
            vec![TaskStatus::Starting, TaskStatus::Failed]
        );
        assert!(
            recorded
                .last_error()
                .expect("error")
                .contains("gentle-shell")
        );
    }

    #[tokio::test]
    async fn fails_without_side_effects_when_workspace_is_missing() {
        let mut lookup: MockWorkspaceLookup = MockWorkspaceLookup::new();
        lookup.expect_find().returning(|_| Ok(None));
        let lifecycle: Arc<TaskLifecycle> = Arc::new(TaskLifecycle::new(
            Arc::new(MockTaskRepository::new()),
            Arc::new(MockTaskEventPublisher::new()),
            Arc::new(lookup),
        ));
        let use_case: CreateTask = CreateTask::new(
            lifecycle,
            Arc::new(MockWorktree::new()),
            Arc::new(MockAgentRuntime::new()),
            Arc::new(MockAgentEventSink::new()),
        );

        let result: Result<Task, TaskError> = use_case.execute(input()).await;

        assert_eq!(
            result,
            Err(TaskError::WorkspaceNotFound("ws-1".to_string()))
        );
    }

    #[tokio::test]
    async fn rejects_empty_prompt() {
        let use_case: CreateTask = CreateTask::new(
            test_support::lifecycle(None, &Recorded::default()),
            Arc::new(MockWorktree::new()),
            Arc::new(MockAgentRuntime::new()),
            Arc::new(MockAgentEventSink::new()),
        );
        let mut empty: NewTask = input();
        empty.prompt = "   ".to_string();

        assert_eq!(use_case.execute(empty).await, Err(TaskError::EmptyPrompt));
    }
}
