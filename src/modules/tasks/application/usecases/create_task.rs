use std::path::PathBuf;
use std::sync::Arc;

use chrono::Utc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::application::usecases::start_task::StartTask;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{
    CreatedTask, NewTask, Task, TaskId, TaskStatus, branch_name, slugify, worktree_dir_name,
};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Crea una tarea en `Backlog` (sin worktree ni agente). Si `start_now` es `true`,
/// delega en [`StartTask`] para arrancarla inmediatamente. Si el arranque falla, la
/// tarea sigue existiendo en `Backlog` y el error se devuelve en `start_error`.
pub struct CreateTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
    start_task: Arc<StartTask>,
}

impl CreateTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        worktrees: Arc<dyn WorktreePort>,
        start_task: Arc<StartTask>,
    ) -> Self {
        Self {
            lifecycle,
            worktrees,
            start_task,
        }
    }

    pub async fn execute(&self, input: NewTask) -> Result<CreatedTask, TaskError> {
        let input: NewTask = input.validated()?;
        let workspace: WorkspaceRef = self.lifecycle.workspace(&input.workspace_id).await?;

        let id: TaskId = TaskId::new();
        let branch: String = branch_name(&id, &input.title);
        let worktree_path: PathBuf = self.worktrees.worktree_path(
            &slugify(&workspace.name),
            &worktree_dir_name(&id, &input.title),
        );

        let now: chrono::DateTime<Utc> = Utc::now();
        let task: Task = Task {
            id,
            workspace_id: workspace.id,
            title: input.title,
            prompt: input.prompt,
            branch,
            base_branch: workspace.base_branch,
            worktree_path,
            status: TaskStatus::Backlog,
            error: None,
            pending_dialogs: Vec::new(),
            created_at: now,
            updated_at: now,
        };
        self.lifecycle.repository().insert(&task).await?;
        self.lifecycle
            .transition(id, TaskStatus::Backlog, None)
            .await?;

        if input.start_now {
            return Ok(match self.start_task.execute(id).await {
                Ok(started) => CreatedTask {
                    task: started,
                    start_error: None,
                },
                Err(error) => CreatedTask {
                    task,
                    start_error: Some(error),
                },
            });
        }
        Ok(CreatedTask {
            task,
            start_error: None,
        })
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;
    use crate::modules::tasks::application::test_support::{self, REPO, Recorded};
    use crate::modules::tasks::domain::models::agent_command::AgentCommand;
    use crate::modules::tasks::mocks::{
        MockAgentEventSink, MockAgentRuntime, MockTaskEventPublisher, MockTaskRepository,
        MockWorkspaceLookup, MockWorktree,
    };

    fn input(start_now: bool) -> NewTask {
        NewTask {
            workspace_id: "ws-1".to_string(),
            title: "Arregla login".to_string(),
            prompt: "Arregla el login roto".to_string(),
            start_now,
        }
    }

    fn worktrees_computing_path_only() -> MockWorktree {
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_worktree_path()
            .returning(|ws, dir| PathBuf::from(format!("/wt/{ws}/{dir}")));
        worktrees.expect_create().never();
        worktrees
    }

    fn no_op_start_task(recorded: &Recorded) -> Arc<StartTask> {
        Arc::new(StartTask::new(
            test_support::lifecycle(None, recorded),
            Arc::new(MockWorktree::new()),
            Arc::new(MockAgentRuntime::new()),
            Arc::new(MockAgentEventSink::new()),
        ))
    }

    fn lifecycle_accepting_insert(recorded: &Recorded) -> Arc<TaskLifecycle> {
        let mut repository: MockTaskRepository = test_support::repository_with(None, recorded);
        repository.expect_insert().times(1).returning(|_| Ok(()));
        test_support::lifecycle_with(repository, recorded)
    }

    #[tokio::test]
    async fn creates_task_in_backlog_without_starting_it() {
        let recorded: Recorded = Recorded::default();
        let use_case: CreateTask = CreateTask::new(
            lifecycle_accepting_insert(&recorded),
            Arc::new(worktrees_computing_path_only()),
            no_op_start_task(&recorded),
        );

        let created: CreatedTask = use_case.execute(input(false)).await.expect("creada");
        let task: Task = created.task;

        assert_eq!(created.start_error, None);
        assert_eq!(task.status, TaskStatus::Backlog);
        assert!(task.pending_dialogs.is_empty());
        assert_eq!(recorded.statuses(), vec![TaskStatus::Backlog]);
        assert!(task.branch.starts_with("pi/arregla-login-"));
    }

    #[tokio::test]
    async fn start_now_delegates_to_start_task() {
        use std::sync::Mutex;

        let recorded: Recorded = Recorded::default();
        let inserted: Arc<Mutex<Option<Task>>> = Arc::new(Mutex::new(None));
        let mut repository: MockTaskRepository = MockTaskRepository::new();
        let insert_cell: Arc<Mutex<Option<Task>>> = inserted.clone();
        repository
            .expect_insert()
            .times(1)
            .returning(move |task: &Task| {
                *insert_cell.lock().expect("lock") = Some(task.clone());
                Ok(())
            });
        let find_cell: Arc<Mutex<Option<Task>>> = inserted.clone();
        repository
            .expect_find()
            .returning(move |_| Ok(find_cell.lock().expect("lock").clone()));
        let statuses: test_support::StatusLog = recorded.statuses.clone();
        repository
            .expect_update_status()
            .returning(move |_, status, error| {
                statuses.lock().expect("lock").push((status, error));
                Ok(())
            });
        repository.expect_append_event().returning(|id, kind| {
            Ok(
                crate::modules::tasks::domain::models::task_event::TaskEvent {
                    task_id: id,
                    seq: Some(1),
                    kind,
                    created_at: Utc::now(),
                },
            )
        });
        repository
            .expect_clear_pending_dialogs()
            .returning(|_| Ok(()));
        repository
            .expect_claim_for_start()
            .times(1)
            .returning(|_, _, _, _| Ok(true));
        let lifecycle: Arc<TaskLifecycle> = test_support::lifecycle_with(repository, &recorded);

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
        let worktrees: Arc<dyn WorktreePort> = Arc::new(worktrees);

        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_start()
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

        let start_task: Arc<StartTask> = Arc::new(StartTask::new(
            lifecycle.clone(),
            worktrees.clone(),
            Arc::new(runtime),
            Arc::new(MockAgentEventSink::new()),
        ));
        let use_case: CreateTask = CreateTask::new(lifecycle, worktrees, start_task);

        let created: CreatedTask = use_case.execute(input(true)).await.expect("creada");

        assert_eq!(created.start_error, None);
        assert_eq!(created.task.status, TaskStatus::Starting);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Backlog]);
    }

    #[tokio::test]
    async fn keeps_created_task_when_start_fails() {
        let recorded: Recorded = Recorded::default();
        // `no_op_start_task` cannot find the task, so starting it fails after the insert.
        let use_case: CreateTask = CreateTask::new(
            lifecycle_accepting_insert(&recorded),
            Arc::new(worktrees_computing_path_only()),
            no_op_start_task(&recorded),
        );

        let created: CreatedTask = use_case.execute(input(true)).await.expect("creada");

        assert_eq!(created.task.status, TaskStatus::Backlog);
        assert!(matches!(created.start_error, Some(TaskError::NotFound(_))));
    }

    #[tokio::test]
    async fn fails_when_insert_fails() {
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository = test_support::repository_with(None, &recorded);
        repository
            .expect_insert()
            .times(1)
            .returning(|_| Err(TaskError::Repository("boom".to_string())));
        let use_case: CreateTask = CreateTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees_computing_path_only()),
            no_op_start_task(&recorded),
        );

        let result: Result<CreatedTask, TaskError> = use_case.execute(input(true)).await;

        assert_eq!(result, Err(TaskError::Repository("boom".to_string())));
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
            lifecycle.clone(),
            Arc::new(MockWorktree::new()),
            Arc::new(StartTask::new(
                lifecycle,
                Arc::new(MockWorktree::new()),
                Arc::new(MockAgentRuntime::new()),
                Arc::new(MockAgentEventSink::new()),
            )),
        );

        let result: Result<CreatedTask, TaskError> = use_case.execute(input(false)).await;

        assert_eq!(
            result,
            Err(TaskError::WorkspaceNotFound("ws-1".to_string()))
        );
    }

    #[tokio::test]
    async fn rejects_empty_prompt() {
        let recorded: Recorded = Recorded::default();
        let use_case: CreateTask = CreateTask::new(
            test_support::lifecycle(None, &recorded),
            Arc::new(MockWorktree::new()),
            no_op_start_task(&recorded),
        );
        let mut empty: NewTask = input(false);
        empty.prompt = "   ".to_string();

        assert_eq!(use_case.execute(empty).await, Err(TaskError::EmptyPrompt));
    }
}
