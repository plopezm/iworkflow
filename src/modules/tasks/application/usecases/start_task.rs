use std::path::PathBuf;
use std::sync::Arc;

use uuid::Uuid;

use chrono::Utc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
use crate::modules::tasks::domain::models::task::{
    Task, TaskId, TaskStatus, branch_name, workspace_dir_name, worktree_dir_name,
};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;
use crate::modules::tasks::domain::ports::outbound::launch_settings_port::LaunchSettingsPort;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
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
    launch_settings: Arc<dyn LaunchSettingsPort>,
    sink: Arc<dyn AgentEventSink>,
    attachments: Arc<dyn TaskAttachmentRepositoryPort>,
    storage: Arc<dyn AttachmentStoragePort>,
}

impl StartTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        worktrees: Arc<dyn WorktreePort>,
        runtime: Arc<dyn AgentRuntimePort>,
        launch_settings: Arc<dyn LaunchSettingsPort>,
        sink: Arc<dyn AgentEventSink>,
        attachments: Arc<dyn TaskAttachmentRepositoryPort>,
        storage: Arc<dyn AttachmentStoragePort>,
    ) -> Self {
        Self {
            lifecycle,
            worktrees,
            runtime,
            launch_settings,
            sink,
            attachments,
            storage,
        }
    }

    pub async fn execute(&self, id: TaskId) -> Result<Task, TaskError> {
        let mut task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_start() {
            return Err(invalid_state("iniciar", task.status));
        }
        let workspace: WorkspaceRef = self.lifecycle.workspace(&task.workspace_id).await?;

        let unique: Uuid = Uuid::new_v4();
        let branch: String = branch_name(&task.title, &unique);
        let worktree_path: PathBuf = self.worktrees.worktree_path(
            &workspace_dir_name(&workspace.name, &workspace.id),
            &worktree_dir_name(&task.title, &unique),
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
            match self.lifecycle.repository().revert_start_claim(id).await {
                Ok(_) => {
                    self.lifecycle
                        .notify_status(id, TaskStatus::Backlog, None)
                        .await?;
                }
                Err(revert_error) => {
                    tracing::warn!(
                        task_id = %id,
                        %revert_error,
                        create_error = %error,
                        "no se pudo revertir la reclamación de arranque; se marca la tarea como fallida"
                    );
                    if let Err(transition_error) = self
                        .lifecycle
                        .transition(id, TaskStatus::Failed, Some(error.to_string()))
                        .await
                    {
                        tracing::warn!(
                            task_id = %id,
                            %transition_error,
                            "no se pudo marcar como fallida la tarea tras el fallo del worktree"
                        );
                    }
                }
            }
            return Err(error);
        }

        self.lifecycle
            .notify_status(id, TaskStatus::Starting, None)
            .await?;
        task.status = TaskStatus::Starting;
        task.updated_at = Utc::now();

        if let Err(error) = self.launch(&task).await {
            let message: String = error.to_string();
            if self.runtime.is_running(id).await
                && let Err(stop_error) = self.runtime.stop(id).await
            {
                tracing::warn!(
                    task_id = %id,
                    %stop_error,
                    "no se pudo detener el agente tras fallar el lanzamiento"
                );
            }
            self.lifecycle
                .transition(id, TaskStatus::Failed, Some(message.clone()))
                .await?;
            task.status = TaskStatus::Failed;
            task.error = Some(message);
        }
        Ok(task)
    }

    /// Lee del almacén las imágenes del prompt inicial; un fichero ausente aborta el arranque.
    async fn load_images(&self, task_id: TaskId) -> Result<Vec<ImageAttachment>, TaskError> {
        let stored: Vec<StoredAttachment> = self.attachments.list_by_task(task_id).await?;
        let mut images: Vec<ImageAttachment> = Vec::with_capacity(stored.len());
        for attachment in stored {
            let data: String = self.storage.read(&attachment.path).await.map_err(|error| {
                TaskError::Storage(format!(
                    "no se pudo leer la imagen adjunta {}: {error}",
                    attachment.name
                ))
            })?;
            images.push(ImageAttachment {
                name: attachment.name,
                mime_type: attachment.mime_type,
                data,
            });
        }
        Ok(images)
    }

    async fn launch(&self, task: &Task) -> Result<(), TaskError> {
        let prepared: LaunchOptions = self.launch_settings.prepare(&task.worktree_path).await?;
        let capabilities: AgentCapabilities =
            self.runtime.runtime_capabilities(prepared.runtime.as_ref());
        let options: LaunchOptions = prepared.restricted_to(&capabilities);
        let images: Vec<ImageAttachment> = self.load_images(task.id).await?;
        self.runtime
            .start(
                task.id,
                &task.worktree_path,
                false,
                &options,
                self.sink.clone(),
            )
            .await?;
        self.runtime
            .send(
                task.id,
                AgentCommand::Prompt {
                    message: task.prompt.clone(),
                    images,
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
    use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
    use crate::modules::tasks::domain::models::launch_options::LaunchModel;
    use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
    use crate::modules::tasks::mocks::{
        MockAgentEventSink, MockAgentRuntime, MockAttachmentStorage, MockLaunchSettings,
        MockTaskAttachmentRepository, MockTaskRepository, MockWorktree,
    };

    fn default_launch_settings() -> Arc<dyn LaunchSettingsPort> {
        let mut launch_settings: MockLaunchSettings = MockLaunchSettings::new();
        launch_settings
            .expect_prepare()
            .returning(|_| Ok(LaunchOptions::default()));
        Arc::new(launch_settings)
    }

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

    fn runtime_not_running() -> MockAgentRuntime {
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime.expect_is_running().returning(|_| false);
        runtime.expect_stop().never();
        runtime
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
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime
            .expect_start()
            .withf(|_, cwd: &Path, resume: &bool, _, _| cwd.starts_with("/wt/demo-ws-1") && !resume)
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        runtime
            .expect_send()
            .withf(|_, command: &AgentCommand| {
                *command
                    == AgentCommand::Prompt {
                        message: "Arregla el login".to_string(),
                        images: Vec::new(),
                    }
            })
            .times(1)
            .returning(|_, _| Ok(()));
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            default_launch_settings(),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Starting);
        assert!(recorded.statuses().is_empty());
    }

    fn png(name: &str) -> ImageAttachment {
        ImageAttachment {
            name: name.to_string(),
            mime_type: "image/png".to_string(),
            data: format!("data-{name}"),
        }
    }

    fn stored(name: &str) -> StoredAttachment {
        StoredAttachment {
            id: format!("id-{name}"),
            name: name.to_string(),
            mime_type: "image/png".to_string(),
            path: format!("t/{name}"),
            size_bytes: 3,
        }
    }

    fn no_attachments() -> Arc<dyn TaskAttachmentRepositoryPort> {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .returning(|_| Ok(Vec::new()));
        Arc::new(attachments)
    }

    fn attachments_listing(names: &[&str]) -> MockTaskAttachmentRepository {
        let listed: Vec<StoredAttachment> = names.iter().map(|n: &&str| stored(n)).collect();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .times(1)
            .returning(move |_| Ok(listed.clone()));
        attachments
    }

    #[tokio::test]
    async fn reads_stored_files_and_sends_images_with_the_initial_prompt() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let images: Vec<ImageAttachment> = vec![png("a.png"), png("b.png")];
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_read()
            .times(2)
            .returning(|path: &str| Ok(format!("data-{}", path.trim_start_matches("t/"))));
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime.expect_start().returning(|_, _, _, _, _| Ok(()));
        runtime
            .expect_send()
            .withf(move |_, command: &AgentCommand| {
                *command
                    == AgentCommand::Prompt {
                        message: "Arregla el login".to_string(),
                        images: images.clone(),
                    }
            })
            .times(1)
            .returning(|_, _| Ok(()));
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            default_launch_settings(),
            Arc::new(MockAgentEventSink::new()),
            Arc::new(attachments_listing(&["a.png", "b.png"])),
            Arc::new(storage),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Starting);
    }

    #[tokio::test]
    async fn marks_task_failed_without_starting_agent_when_images_cannot_be_loaded() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .times(1)
            .returning(|_| Err(TaskError::Repository("adjuntos ilegibles".to_string())));
        let mut runtime: MockAgentRuntime = runtime_not_running();
        runtime.expect_start().never();
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            default_launch_settings(),
            Arc::new(MockAgentEventSink::new()),
            Arc::new(attachments),
            Arc::new(MockAttachmentStorage::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Failed);
        assert!(
            recorded
                .last_error()
                .expect("error")
                .contains("adjuntos ilegibles")
        );
    }

    #[tokio::test]
    async fn marks_task_failed_without_starting_agent_when_an_image_file_is_missing() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_read()
            .times(1)
            .returning(|_| Err(TaskError::Storage("no existe".to_string())));
        let mut runtime: MockAgentRuntime = runtime_not_running();
        runtime.expect_start().never();
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            default_launch_settings(),
            Arc::new(MockAgentEventSink::new()),
            Arc::new(attachments_listing(&["a.png"])),
            Arc::new(storage),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Failed);
        let error: String = recorded.last_error().expect("error");
        assert!(
            error.contains("a.png") && error.contains("no existe"),
            "{error}"
        );
    }

    #[tokio::test]
    async fn marks_task_failed_when_agent_cannot_start() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime.expect_start().returning(|_, _, _, _, _| {
            Err(TaskError::Runtime("no existe gentle-shell".to_string()))
        });
        runtime.expect_is_running().returning(|_| false);
        runtime.expect_stop().never();
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            default_launch_settings(),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
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
            Arc::new(MockLaunchSettings::new()),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        let result: Result<Task, TaskError> = use_case.execute(id).await;

        assert_eq!(result, Err(TaskError::Git("no space left".to_string())));
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn returns_create_error_and_marks_failed_when_revert_also_fails() {
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
            .returning(|_| Err(TaskError::Repository("db bloqueada".to_string())));
        let use_case: StartTask = StartTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            Arc::new(MockAgentRuntime::new()),
            Arc::new(MockLaunchSettings::new()),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        let result: Result<Task, TaskError> = use_case.execute(id).await;

        assert_eq!(result, Err(TaskError::Git("no space left".to_string())));
        assert_eq!(recorded.statuses(), vec![TaskStatus::Failed]);
        assert!(
            recorded
                .last_error()
                .expect("error")
                .contains("no space left")
        );
    }

    #[tokio::test]
    async fn stops_runtime_when_prompt_send_fails_after_start() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime
            .expect_start()
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        runtime
            .expect_send()
            .times(1)
            .returning(|_, _| Err(TaskError::Runtime("pipe roto".to_string())));
        runtime.expect_is_running().times(1).returning(|_| true);
        runtime.expect_stop().times(1).returning(|_| Ok(()));
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            default_launch_settings(),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Failed);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Failed]);
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
            Arc::new(MockLaunchSettings::new()),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
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
            Arc::new(MockLaunchSettings::new()),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        )
        .execute(id)
        .await;

        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }

    #[tokio::test]
    async fn passes_prepared_model_to_runtime_start() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime
            .expect_start()
            .withf(|_, _, _, options: &LaunchOptions, _| {
                options.model
                    == Some(LaunchModel {
                        provider: "anthropic".to_string(),
                        model: "claude-5".to_string(),
                    })
            })
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        runtime.expect_send().returning(|_, _| Ok(()));
        let mut launch_settings: MockLaunchSettings = MockLaunchSettings::new();
        launch_settings.expect_prepare().returning(|_| {
            Ok(LaunchOptions {
                model: Some(LaunchModel {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                }),
                thinking: None,
                ..Default::default()
            })
        });
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            Arc::new(launch_settings),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Starting);
    }

    #[tokio::test]
    async fn passes_prepared_thinking_level_to_runtime_start() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities::full());
        runtime
            .expect_start()
            .withf(|_, _, _, options: &LaunchOptions, _| {
                options.thinking == Some("high".to_string())
            })
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        runtime.expect_send().returning(|_, _| Ok(()));
        let mut launch_settings: MockLaunchSettings = MockLaunchSettings::new();
        launch_settings.expect_prepare().returning(|_| {
            Ok(LaunchOptions {
                model: None,
                thinking: Some("high".to_string()),
                ..Default::default()
            })
        });
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            Arc::new(launch_settings),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Starting);
    }

    #[tokio::test]
    async fn strips_model_and_thinking_the_runtime_does_not_support() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_runtime_capabilities()
            .times(1)
            .returning(|_| AgentCapabilities::minimal());
        runtime
            .expect_start()
            .withf(|_, _, _, options: &LaunchOptions, _| {
                options.model.is_none() && options.thinking.is_none()
            })
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        runtime.expect_send().returning(|_, _| Ok(()));
        let mut launch_settings: MockLaunchSettings = MockLaunchSettings::new();
        launch_settings.expect_prepare().returning(|_| {
            Ok(LaunchOptions {
                model: Some(LaunchModel {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                }),
                thinking: Some("high".to_string()),
                ..Default::default()
            })
        });
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            Arc::new(launch_settings),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Starting);
    }

    #[tokio::test]
    async fn keeps_only_the_option_the_runtime_supports() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut runtime: MockAgentRuntime = MockAgentRuntime::new();
        runtime
            .expect_runtime_capabilities()
            .returning(|_| AgentCapabilities {
                model_selection: true,
                ..AgentCapabilities::minimal()
            });
        runtime
            .expect_start()
            .withf(|_, _, _, options: &LaunchOptions, _| {
                options.model.is_some() && options.thinking.is_none()
            })
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        runtime.expect_send().returning(|_, _| Ok(()));
        let mut launch_settings: MockLaunchSettings = MockLaunchSettings::new();
        launch_settings.expect_prepare().returning(|_| {
            Ok(LaunchOptions {
                model: Some(LaunchModel {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                }),
                thinking: Some("high".to_string()),
                ..Default::default()
            })
        });
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime),
            Arc::new(launch_settings),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        use_case.execute(id).await.expect("iniciada");
    }

    #[tokio::test]
    async fn marks_task_failed_when_launch_settings_prepare_fails() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut launch_settings: MockLaunchSettings = MockLaunchSettings::new();
        launch_settings.expect_prepare().returning(|_| {
            Err(TaskError::Runtime(
                "pin de perfil inv\u{e1}lido".to_string(),
            ))
        });
        let use_case: StartTask = StartTask::new(
            lifecycle_accepting_plan(task, &recorded),
            Arc::new(worktrees()),
            Arc::new(runtime_not_running()),
            Arc::new(launch_settings),
            Arc::new(MockAgentEventSink::new()),
            no_attachments(),
            Arc::new(MockAttachmentStorage::new()),
        );

        let task: Task = use_case.execute(id).await.expect("iniciada");

        assert_eq!(task.status, TaskStatus::Failed);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Failed]);
        assert!(
            recorded
                .last_error()
                .expect("error")
                .contains("pin de perfil")
        );
    }
}
