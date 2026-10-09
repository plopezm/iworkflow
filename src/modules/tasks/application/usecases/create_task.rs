use std::path::PathBuf;
use std::sync::Arc;

use uuid::Uuid;

use chrono::Utc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::application::usecases::start_task::StartTask;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{
    CreatedTask, NewTask, Task, TaskId, TaskStatus, branch_name, workspace_dir_name,
    worktree_dir_name,
};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Crea una tarea en `Backlog` (sin worktree ni agente). Si `start_now` es `true`,
/// delega en [`StartTask`] para arrancarla inmediatamente. Si el arranque falla, la
/// tarea sigue existiendo en `Backlog` y el error se devuelve en `start_error`.
pub struct CreateTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
    start_task: Arc<StartTask>,
    attachments: Arc<dyn TaskAttachmentRepositoryPort>,
}

impl CreateTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        worktrees: Arc<dyn WorktreePort>,
        start_task: Arc<StartTask>,
        attachments: Arc<dyn TaskAttachmentRepositoryPort>,
    ) -> Self {
        Self {
            lifecycle,
            worktrees,
            start_task,
            attachments,
        }
    }

    pub async fn execute(&self, input: NewTask) -> Result<CreatedTask, TaskError> {
        let input: NewTask = input.validated()?;
        let workspace: WorkspaceRef = self.lifecycle.workspace(&input.workspace_id).await?;

        let id: TaskId = TaskId::new();
        let unique: Uuid = Uuid::new_v4();
        let branch: String = branch_name(&input.title, &unique);
        let worktree_path: PathBuf = self.worktrees.worktree_path(
            &workspace_dir_name(&workspace.name, &workspace.id),
            &worktree_dir_name(&input.title, &unique),
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
            priority: input.priority,
            error: None,
            pending_dialogs: Vec::new(),
            subagents: Default::default(),
            created_at: now,
            updated_at: now,
        };
        self.lifecycle.repository().insert(&task).await?;
        if !input.images.is_empty()
            && let Err(error) = self.attachments.insert_many(id, &input.images).await
        {
            if let Err(cleanup_error) = self.lifecycle.repository().delete_if_backlog(id).await {
                tracing::warn!(
                    task_id = %id,
                    %cleanup_error,
                    save_error = %error,
                    "no se pudo borrar la tarea tras fallar el guardado de sus imágenes"
                );
            }
            return Err(error);
        }
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
    use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
    use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
    use crate::modules::tasks::domain::models::task_priority::TaskPriority;
    use crate::modules::tasks::domain::ports::outbound::launch_settings_port::LaunchSettingsPort;
    use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
    use crate::modules::tasks::mocks::{
        MockAgentEventSink, MockAgentRuntime, MockLaunchSettings, MockTaskAttachmentRepository,
        MockTaskEventPublisher, MockTaskRepository, MockWorkspaceLookup, MockWorktree,
    };

    fn default_launch_settings() -> Arc<dyn LaunchSettingsPort> {
        let mut launch_settings: MockLaunchSettings = MockLaunchSettings::new();
        launch_settings
            .expect_prepare()
            .returning(|_| Ok(LaunchOptions::default()));
        Arc::new(launch_settings)
    }

    fn input(start_now: bool) -> NewTask {
        NewTask {
            workspace_id: "ws-1".to_string(),
            title: "Arregla login".to_string(),
            prompt: "Arregla el login roto".to_string(),
            priority: TaskPriority::default(),
            images: Vec::new(),
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
            Arc::new(MockLaunchSettings::new()),
            Arc::new(MockAgentEventSink::new()),
            Arc::new(MockTaskAttachmentRepository::new()),
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
            Arc::new(MockTaskAttachmentRepository::new()),
        );

        let created: CreatedTask = use_case.execute(input(false)).await.expect("creada");
        let task: Task = created.task;

        assert_eq!(created.start_error, None);
        assert_eq!(task.status, TaskStatus::Backlog);
        assert!(task.pending_dialogs.is_empty());
        assert_eq!(recorded.statuses(), vec![TaskStatus::Backlog]);
        assert!(task.branch.starts_with("pi/arregla-login-"));
    }

    fn png(name: &str) -> ImageAttachment {
        ImageAttachment {
            name: name.to_string(),
            mime_type: "image/png".to_string(),
            data: "aGk=".to_string(),
        }
    }

    #[tokio::test]
    async fn stores_attached_images_right_after_inserting_the_task() {
        let recorded: Recorded = Recorded::default();
        let images: Vec<ImageAttachment> = vec![png("a.png"), png("b.png")];
        let expected: Vec<ImageAttachment> = images.clone();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_insert_many()
            .withf(move |_, stored: &[ImageAttachment]| stored == expected.as_slice())
            .times(1)
            .returning(|_, _| Ok(()));
        let use_case: CreateTask = CreateTask::new(
            lifecycle_accepting_insert(&recorded),
            Arc::new(worktrees_computing_path_only()),
            no_op_start_task(&recorded),
            Arc::new(attachments),
        );
        let with_images: NewTask = NewTask {
            images,
            ..input(false)
        };

        let created: CreatedTask = use_case.execute(with_images).await.expect("creada");

        assert_eq!(created.task.status, TaskStatus::Backlog);
    }

    #[tokio::test]
    async fn skips_attachment_storage_when_there_are_no_images() {
        let recorded: Recorded = Recorded::default();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments.expect_insert_many().never();
        let use_case: CreateTask = CreateTask::new(
            lifecycle_accepting_insert(&recorded),
            Arc::new(worktrees_computing_path_only()),
            no_op_start_task(&recorded),
            Arc::new(attachments),
        );

        use_case.execute(input(false)).await.expect("creada");
    }

    #[tokio::test]
    async fn deletes_the_task_and_propagates_when_storing_images_fails() {
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository = test_support::repository_with(None, &recorded);
        repository.expect_insert().times(1).returning(|_| Ok(()));
        repository
            .expect_delete_if_backlog()
            .times(1)
            .returning(|_| Ok(true));
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_insert_many()
            .times(1)
            .returning(|_, _| Err(TaskError::Repository("disco lleno".to_string())));
        let use_case: CreateTask = CreateTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees_computing_path_only()),
            no_op_start_task(&recorded),
            Arc::new(attachments),
        );
        let with_images: NewTask = NewTask {
            images: vec![png("a.png")],
            ..input(true)
        };

        let result: Result<CreatedTask, TaskError> = use_case.execute(with_images).await;

        assert_eq!(
            result,
            Err(TaskError::Repository("disco lleno".to_string()))
        );
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn rejects_invalid_images_before_touching_the_repository() {
        let recorded: Recorded = Recorded::default();
        let use_case: CreateTask = CreateTask::new(
            test_support::lifecycle(None, &recorded),
            Arc::new(MockWorktree::new()),
            no_op_start_task(&recorded),
            Arc::new(MockTaskAttachmentRepository::new()),
        );
        let invalid: NewTask = NewTask {
            images: vec![ImageAttachment {
                mime_type: "image/bmp".to_string(),
                ..png("a.bmp")
            }],
            ..input(false)
        };

        let result: Result<CreatedTask, TaskError> = use_case.execute(invalid).await;

        assert!(matches!(result, Err(TaskError::InvalidAttachment(_))));
    }

    #[tokio::test]
    async fn keeps_requested_priority_on_the_created_task() {
        let recorded: Recorded = Recorded::default();
        let use_case: CreateTask = CreateTask::new(
            lifecycle_accepting_insert(&recorded),
            Arc::new(worktrees_computing_path_only()),
            no_op_start_task(&recorded),
            Arc::new(MockTaskAttachmentRepository::new()),
        );
        let urgent: NewTask = NewTask {
            priority: TaskPriority::Urgent,
            ..input(false)
        };

        let created: CreatedTask = use_case.execute(urgent).await.expect("creada");

        assert_eq!(created.task.priority, TaskPriority::Urgent);
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
            .returning(|_, _, _, _, _| Ok(()));
        runtime
            .expect_send()
            .withf(|_, command: &AgentCommand| {
                *command
                    == AgentCommand::Prompt {
                        message: "Arregla el login roto".to_string(),
                        images: Vec::new(),
                    }
            })
            .times(1)
            .returning(|_, _| Ok(()));

        let mut attachments_mock: MockTaskAttachmentRepository =
            MockTaskAttachmentRepository::new();
        attachments_mock
            .expect_list_by_task()
            .returning(|_| Ok(Vec::new()));
        let attachments: Arc<dyn TaskAttachmentRepositoryPort> = Arc::new(attachments_mock);
        let start_task: Arc<StartTask> = Arc::new(StartTask::new(
            lifecycle.clone(),
            worktrees.clone(),
            Arc::new(runtime),
            default_launch_settings(),
            Arc::new(MockAgentEventSink::new()),
            attachments.clone(),
        ));
        let use_case: CreateTask = CreateTask::new(lifecycle, worktrees, start_task, attachments);

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
            Arc::new(MockTaskAttachmentRepository::new()),
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
            Arc::new(MockTaskAttachmentRepository::new()),
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
                Arc::new(MockLaunchSettings::new()),
                Arc::new(MockAgentEventSink::new()),
                Arc::new(MockTaskAttachmentRepository::new()),
            )),
            Arc::new(MockTaskAttachmentRepository::new()),
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
            Arc::new(MockTaskAttachmentRepository::new()),
        );
        let mut empty: NewTask = input(false);
        empty.prompt = "   ".to_string();

        assert_eq!(use_case.execute(empty).await, Err(TaskError::EmptyPrompt));
    }
}
