use std::path::PathBuf;
use std::sync::Arc;

use chrono::Utc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::task::{
    Task, TaskId, TaskStatus, branch_name, slugify, worktree_dir_name,
};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Arranca una tarea en `Backlog`: crea el worktree, lanza el agente y le envía el
/// prompt inicial. Si el agente no arranca, la tarea queda en `Failed` (el worktree
/// se conserva). La tarea se reclama atómicamente (`Backlog` -> `Starting`), de modo que
/// dos arranques concurrentes no pueden procesarla a la vez. Si crear el worktree
/// falla, se revierte la reclamación (vuelve a `Backlog`) y el error se propaga.
pub struct StartTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
    runtime: Arc<dyn AgentRuntimePort>,
    sink: Arc<dyn AgentEventSink>,
}

impl StartTask {
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

    pub async fn execute(&self, id: TaskId) -> Result<Task, TaskError> {
        let mut task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_start() {
            return Err(invalid_state("iniciar", task.status));
        }
        let workspace: WorkspaceRef = self.lifecycle.workspace(&task.workspace_id).await?;

        let branch: String = branch_name(&task.id, &task.title);
        let worktree_path: PathBuf = self.worktrees.worktree_path(
            &slugify(&workspace.name),
            &worktree_dir_name(&task.id, &task.title),
        );
        let base_branch: String = workspace.base_branch.clone();

        let claimed: bool = self
            .lifecycle
            .repository()
            .claim_for_start(
                id,
                branch.clone(),
                worktree_path.clone(),
                base_branch.clone(),
            )
            .await?;
        if !claimed {
            return Err(self.lifecycle.conflict(id, "iniciar").await);
        }
        task.branch = branch;
        task.worktree_path = worktree_path;
        task.base_branch = base_branch;

        if let Err(error) = self
            .worktrees
            .create(
                &workspace.path,
                &task.base_branch,
                &task.branch,
                &task.worktree_path,
            )
            .await
        {
            self.lifecycle.repository().revert_start_claim(id).await?;
            self.lifecycle
                .notify_status(id, TaskStatus::Backlog, None)
                .await?;
            return Err(error);
        }

        self.lifecycle
            .notify_status(id, TaskStatus::Starting, None)
            .await?;
        task.status = TaskStatus::Starting;
        task.updated_at = Utc::now();

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
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::{
        MockAgentEventSink, MockAgentRuntime, MockTaskRepository, MockWorktree,
    };

    fn worktrees() -> MockWorktree {
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_worktree_path()
            .returning(|ws, dir| PathBuf::from(format!("/wt/{ws}/{dir}")));
        worktrees
            .expect_create()
            .withf(|repo: &Path, base: &str, branch: &str, _| {
                repo == Path::new(test_support::REPO)
                    && base == "main"
                    && branch.starts_with("pi/arregla-el-login-")
            })
            .times(1)
            .returning(|_, _, _, _| Ok(()));
        worktrees
    }

    fn lifecycle_accepting_plan(task: Task, recorded: &Recorded) -> Arc<TaskLifecycle> {
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), recorded);
        repository
            .expect_claim_for_start()
            .times(1)
            .returning(|_, _, _, _| Ok(true));
        test_support::lifecycle_with(repository, recorded)
    }

    #[tokio::test]
    async fn creates_worktree_starts_agent_and_sends_prompt() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
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
                        message: "Arregla el login".to_string(),
                    }
            })
            .times(1)
            .returning(|_, _| Ok(()));
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            Arc::new(MockAgentEventSink::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Starting);
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn marks_task_failed_when_agent_cannot_start() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_start()
            .returning(|_, _, _, _| Err(TaskError::Runtime("no existe gentle-shell".to_string())));
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            Arc::new(MockAgentEventSink::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Failed);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Failed]);
        assert!(
            recorded
                .last_error()
                .expect("error")
                .contains("gentle-shell")
        );
    }

    #[tokio::test]
    async fn worktree_creation_failure_leaves_task_in_backlog() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_worktree_path()
            .returning(|ws, dir| PathBuf::from(format!("/wt/{ws}/{dir}")));
        worktrees
            .expect_create()
            .returning(|_, _, _, _| Err(TaskError::Git("no space left".to_string())));
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_claim_for_start()
            .times(1)
            .returning(|_, _, _, _| Ok(true));
        repository
            .expect_revert_start_claim()
            .times(1)
            .returning(|_| Ok(true));
        let use_case: StartTask = StartTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            Arc::new(MockAgentRuntime::new()),
            Arc::new(MockAgentEventSink::new()),
        );

        let result: Result<Task, TaskError> = use_case.execute(id).await;

        assert_eq!(result, Err(TaskError::Git("no space left".to_string())));
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn rejects_when_claim_is_lost_to_a_concurrent_start() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_claim_for_start()
            .times(1)
            .returning(|_, _, _, _| Ok(false));
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_worktree_path()
            .returning(|ws, dir| PathBuf::from(format!("/wt/{ws}/{dir}")));
        let use_case: StartTask = StartTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            Arc::new(MockAgentRuntime::new()),
            Arc::new(MockAgentEventSink::new()),
        );

        let result: Result<Task, TaskError> = use_case.execute(id).await;

        assert!(result.is_err());
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn rejects_task_not_in_backlog() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let result: Result<Task, TaskError> = StartTask::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(MockWorktree::new()),
            Arc::new(MockAgentRuntime::new()),
            Arc::new(MockAgentEventSink::new()),
        )
        .execute(id)
        .await;

        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
