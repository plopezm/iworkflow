use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, MutexGuard};

use crate::core::db::pool::DbPool;
use crate::modules::tasks::application::services::queued_agent_event_sink::{
    DEFAULT_QUEUE_CAPACITY, QueuedAgentEventSink,
};
use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::application::usecases::abort_task::AbortTask;
use crate::modules::tasks::application::usecases::answer_dialog::AnswerDialog;
use crate::modules::tasks::application::usecases::create_task::CreateTask;
use crate::modules::tasks::application::services::attachment_cleanup::AttachmentCleanup;
use crate::modules::tasks::application::usecases::delete_task::DeleteTask;
use crate::modules::tasks::application::usecases::delete_workspace_tasks::DeleteWorkspaceTasks;
use crate::modules::tasks::application::usecases::discard_task::DiscardTask;
use crate::modules::tasks::application::usecases::finish_task::FinishTask;
use crate::modules::tasks::application::usecases::list_task_attachments::ListTaskAttachments;
use crate::modules::tasks::application::usecases::get_task::GetTask;
use crate::modules::tasks::application::usecases::get_task_attachment::GetTaskAttachment;
use crate::modules::tasks::application::usecases::get_task_diff::GetTaskDiff;
use crate::modules::tasks::application::usecases::get_task_events::GetTaskEvents;
use crate::modules::tasks::application::usecases::handle_agent_event::HandleAgentEvent;
use crate::modules::tasks::application::usecases::list_done_tasks::ListDoneTasks;
use crate::modules::tasks::application::usecases::list_tasks::ListTasks;
use crate::modules::tasks::application::usecases::list_workspace_options::ListWorkspaceOptions;
use crate::modules::tasks::application::usecases::merge_task::MergeTask;
use crate::modules::tasks::application::usecases::reconcile_tasks::ReconcileTasks;
use crate::modules::tasks::application::usecases::reset_task::ResetTask;
use crate::modules::tasks::application::usecases::resume_task::ResumeTask;
use crate::modules::tasks::application::usecases::send_message::SendMessage;
use crate::modules::tasks::application::usecases::start_task::StartTask;
use crate::modules::tasks::application::usecases::update_task::UpdateTask;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::runtime_id::RuntimeId;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;
use crate::modules::tasks::domain::ports::outbound::launch_settings_port::LaunchSettingsPort;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::task_event_publisher_port::TaskEventPublisherPort;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;
use crate::modules::tasks::infrastructure::adapters::outbound::events::broadcast_task_event_bus::BroadcastTaskEventBus;
use crate::modules::tasks::infrastructure::adapters::outbound::git::caching_worktree::CachingWorktree;
use crate::modules::tasks::infrastructure::adapters::outbound::git::git_cli_worktree_adapter::GitCliWorktreeAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::retrying_task_repository::{
    RetryPolicy, RetryingTaskRepository,
};
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::task_attachment_repository_adapter::TaskAttachmentRepositoryAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::TaskRepositoryAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::storage::fs_attachment_storage_adapter::FsAttachmentStorageAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::pi::pi_rpc_runtime_adapter::PiRpcRuntimeAdapter;

/// Identificador del runtime `pi` en `LaunchOptions::runtime`.
const PI_RUNTIME: &str = "pi";

/// Puente temporal: elige entre dos adaptadores según `LaunchOptions::runtime` (por defecto
/// gentle-shell) y recuerda la elección por tarea para delegar el resto de operaciones.
struct TemporaryRuntimeSelector {
    gentle_shell: Arc<dyn AgentRuntimePort>,
    pi: Arc<dyn AgentRuntimePort>,
    chosen: Mutex<HashMap<TaskId, Arc<dyn AgentRuntimePort>>>,
}

impl TemporaryRuntimeSelector {
    fn new(gentle_shell: Arc<dyn AgentRuntimePort>, pi: Arc<dyn AgentRuntimePort>) -> Self {
        Self {
            gentle_shell,
            pi,
            chosen: Mutex::new(HashMap::new()),
        }
    }

    fn adapter_for(&self, options: &LaunchOptions) -> Arc<dyn AgentRuntimePort> {
        match options.runtime.as_ref().map(RuntimeId::as_str) {
            Some(PI_RUNTIME) => self.pi.clone(),
            _ => self.gentle_shell.clone(),
        }
    }

    fn chosen(&self, task_id: TaskId) -> Result<Option<Arc<dyn AgentRuntimePort>>, TaskError> {
        let chosen: MutexGuard<'_, HashMap<TaskId, Arc<dyn AgentRuntimePort>>> = self
            .chosen
            .lock()
            .map_err(|_| TaskError::Runtime("registro de runtimes corrupto".to_string()))?;
        Ok(chosen.get(&task_id).cloned())
    }
}

#[async_trait::async_trait]
impl AgentRuntimePort for TemporaryRuntimeSelector {
    async fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        resume: bool,
        options: &LaunchOptions,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError> {
        let adapter: Arc<dyn AgentRuntimePort> = self.adapter_for(options);
        adapter.start(task_id, cwd, resume, options, sink).await?;
        self.chosen
            .lock()
            .map_err(|_| TaskError::Runtime("registro de runtimes corrupto".to_string()))?
            .insert(task_id, adapter);
        Ok(())
    }

    async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError> {
        match self.chosen(task_id)? {
            Some(adapter) => adapter.send(task_id, command).await,
            None => Err(TaskError::AgentNotRunning),
        }
    }

    async fn stop(&self, task_id: TaskId) -> Result<(), TaskError> {
        match self.chosen(task_id)? {
            Some(adapter) => adapter.stop(task_id).await,
            None => Ok(()),
        }
    }

    async fn is_running(&self, task_id: TaskId) -> bool {
        match self.chosen(task_id) {
            Ok(Some(adapter)) => adapter.is_running(task_id).await,
            _ => false,
        }
    }

    fn capabilities(&self, task_id: TaskId) -> AgentCapabilities {
        match self.chosen(task_id) {
            Ok(Some(adapter)) => adapter.capabilities(task_id),
            _ => AgentCapabilities::minimal(),
        }
    }
}

/// Configuración propia del feature de tareas.
#[derive(Debug, Clone)]
pub struct TasksSettings {
    pub gentle_shell_bin: String,
    pub pi_bin: String,
    pub worktrees_dir: PathBuf,
    pub attachments_dir: PathBuf,
}

#[derive(Clone)]
pub struct TasksContext {
    pub create_task: Arc<CreateTask>,
    pub start_task: Arc<StartTask>,
    pub update_task: Arc<UpdateTask>,
    pub delete_task: Arc<DeleteTask>,
    pub delete_workspace_tasks: Arc<DeleteWorkspaceTasks>,
    pub send_message: Arc<SendMessage>,
    pub abort_task: Arc<AbortTask>,
    pub answer_dialog: Arc<AnswerDialog>,
    pub finish_task: Arc<FinishTask>,
    pub resume_task: Arc<ResumeTask>,
    pub merge_task: Arc<MergeTask>,
    pub discard_task: Arc<DiscardTask>,
    pub get_task: Arc<GetTask>,
    pub list_task_attachments: Arc<ListTaskAttachments>,
    pub get_task_attachment: Arc<GetTaskAttachment>,
    pub get_task_diff: Arc<GetTaskDiff>,
    pub get_task_events: Arc<GetTaskEvents>,
    pub list_tasks: Arc<ListTasks>,
    pub list_done_tasks: Arc<ListDoneTasks>,
    pub list_workspace_options: Arc<ListWorkspaceOptions>,
    pub reconcile_tasks: Arc<ReconcileTasks>,
    pub reset_task: Arc<ResetTask>,
    /// Bus en vivo consumido por el endpoint SSE.
    pub event_bus: BroadcastTaskEventBus,
    /// Sink de eventos de agente (cola con reintento persistente hacia SQLite).
    pub event_sink: Arc<dyn AgentEventSink>,
    /// Misma instancia que `event_sink`, para poder vaciarla en el apagado.
    event_queue: Arc<QueuedAgentEventSink>,
}

impl TasksContext {
    /// Requiere un runtime de tokio activo: lanza el worker de la cola de eventos.
    pub fn new(
        pool: DbPool,
        settings: TasksSettings,
        workspaces: Arc<dyn WorkspaceLookupPort>,
        launch_settings: Arc<dyn LaunchSettingsPort>,
    ) -> Self {
        let gentle_shell: Arc<dyn AgentRuntimePort> =
            Arc::new(PiRpcRuntimeAdapter::new(settings.gentle_shell_bin));
        let pi: Arc<dyn AgentRuntimePort> = Arc::new(PiRpcRuntimeAdapter::new(settings.pi_bin));
        let runtime: Arc<dyn AgentRuntimePort> =
            Arc::new(TemporaryRuntimeSelector::new(gentle_shell, pi));
        Self::with_runtime(
            pool,
            settings.worktrees_dir,
            settings.attachments_dir,
            workspaces,
            runtime,
            launch_settings,
        )
    }

    /// Permite sustituir el runtime de agentes (tests de extremo a extremo).
    ///
    /// Requiere un runtime de tokio activo: lanza el worker de la cola de eventos.
    pub fn with_runtime(
        pool: DbPool,
        worktrees_dir: PathBuf,
        attachments_dir: PathBuf,
        workspaces: Arc<dyn WorkspaceLookupPort>,
        runtime: Arc<dyn AgentRuntimePort>,
        launch_settings: Arc<dyn LaunchSettingsPort>,
    ) -> Self {
        let attachments: Arc<dyn TaskAttachmentRepositoryPort> =
            Arc::new(TaskAttachmentRepositoryAdapter::new(pool.clone()));
        let storage: Arc<dyn AttachmentStoragePort> =
            Arc::new(FsAttachmentStorageAdapter::new(attachments_dir));
        let base_repository: Arc<dyn TaskRepositoryPort> =
            Arc::new(TaskRepositoryAdapter::new(pool));
        let repository: Arc<dyn TaskRepositoryPort> = Arc::new(RetryingTaskRepository::new(
            base_repository.clone(),
            RetryPolicy::bounded(),
        ));
        let events_repository: Arc<dyn TaskRepositoryPort> = Arc::new(RetryingTaskRepository::new(
            base_repository,
            RetryPolicy::persistent(),
        ));
        let event_bus: BroadcastTaskEventBus = BroadcastTaskEventBus::new();
        let publisher: Arc<dyn TaskEventPublisherPort> = Arc::new(event_bus.clone());
        let worktrees: Arc<dyn WorktreePort> = Arc::new(GitCliWorktreeAdapter::new(worktrees_dir));
        let diff_worktrees: Arc<dyn WorktreePort> =
            Arc::new(CachingWorktree::new(worktrees.clone()));
        let lifecycle: Arc<TaskLifecycle> = Arc::new(TaskLifecycle::new(
            repository.clone(),
            publisher.clone(),
            workspaces.clone(),
        ));
        let events_lifecycle: Arc<TaskLifecycle> = Arc::new(TaskLifecycle::new(
            events_repository,
            publisher,
            workspaces.clone(),
        ));
        let handler: Arc<dyn AgentEventSink> = Arc::new(HandleAgentEvent::new(events_lifecycle));
        let event_queue: Arc<QueuedAgentEventSink> =
            Arc::new(QueuedAgentEventSink::new(handler, DEFAULT_QUEUE_CAPACITY));
        let sink: Arc<dyn AgentEventSink> = event_queue.clone();
        let start_task: Arc<StartTask> = Arc::new(StartTask::new(
            lifecycle.clone(),
            worktrees.clone(),
            runtime.clone(),
            launch_settings.clone(),
            sink.clone(),
            attachments.clone(),
            storage.clone(),
        ));

        Self {
            create_task: Arc::new(CreateTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                start_task.clone(),
                attachments.clone(),
                storage.clone(),
            )),
            start_task,
            update_task: Arc::new(UpdateTask::new(
                lifecycle.clone(),
                attachments.clone(),
                storage.clone(),
            )),
            delete_task: Arc::new(DeleteTask::new(lifecycle.clone(), storage.clone())),
            delete_workspace_tasks: Arc::new(DeleteWorkspaceTasks::new(
                lifecycle.clone(),
                worktrees.clone(),
                storage.clone(),
            )),
            send_message: Arc::new(SendMessage::new(lifecycle.clone(), runtime.clone())),
            abort_task: Arc::new(AbortTask::new(lifecycle.clone(), runtime.clone())),
            answer_dialog: Arc::new(AnswerDialog::new(lifecycle.clone(), runtime.clone())),
            finish_task: Arc::new(FinishTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime.clone(),
                Arc::new(AttachmentCleanup::new(attachments.clone(), storage.clone())),
            )),
            resume_task: Arc::new(ResumeTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime.clone(),
                launch_settings,
                sink.clone(),
            )),
            merge_task: Arc::new(MergeTask::new(lifecycle.clone(), worktrees.clone())),
            discard_task: Arc::new(DiscardTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime.clone(),
            )),
            reset_task: Arc::new(ResetTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime,
            )),
            get_task: Arc::new(GetTask::new(lifecycle.clone())),
            list_task_attachments: Arc::new(ListTaskAttachments::new(attachments.clone())),
            get_task_attachment: Arc::new(GetTaskAttachment::new(attachments, storage)),
            get_task_diff: Arc::new(GetTaskDiff::new(lifecycle.clone(), diff_worktrees)),
            get_task_events: Arc::new(GetTaskEvents::new(repository.clone())),
            list_tasks: Arc::new(ListTasks::new(repository.clone())),
            list_done_tasks: Arc::new(ListDoneTasks::new(repository)),
            list_workspace_options: Arc::new(ListWorkspaceOptions::new(workspaces)),
            reconcile_tasks: Arc::new(ReconcileTasks::new(lifecycle, worktrees)),
            event_bus,
            event_sink: sink,
            event_queue,
        }
    }

    /// Espera a que se persistan todos los eventos de agente ya aceptados.
    pub async fn flush_events(&self) {
        self.event_queue.flush().await;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::MockAgentRuntime;

    fn started_mock() -> MockAgentRuntime {
        let mut mock: MockAgentRuntime = MockAgentRuntime::new();
        mock.expect_start()
            .times(1)
            .returning(|_, _, _, _, _| Ok(()));
        mock
    }

    fn untouched_mock() -> MockAgentRuntime {
        let mut mock: MockAgentRuntime = MockAgentRuntime::new();
        mock.expect_start().never();
        mock
    }

    async fn start_with(selector: &TemporaryRuntimeSelector, runtime: Option<&str>) {
        let options: LaunchOptions = LaunchOptions {
            runtime: runtime.map(|r: &str| RuntimeId::parse(r).expect("runtime válido")),
            ..LaunchOptions::default()
        };
        let sink: Arc<dyn AgentEventSink> =
            Arc::new(crate::modules::tasks::mocks::MockAgentEventSink::new());
        selector
            .start(TaskId::new(), Path::new("/tmp"), false, &options, sink)
            .await
            .expect("start");
    }

    #[tokio::test]
    async fn pi_runtime_selects_the_pi_adapter() {
        let selector: TemporaryRuntimeSelector =
            TemporaryRuntimeSelector::new(Arc::new(untouched_mock()), Arc::new(started_mock()));
        start_with(&selector, Some("pi")).await;
    }

    #[tokio::test]
    async fn default_and_gentle_shell_select_the_gentle_shell_adapter() {
        for runtime in [None, Some("gentle-shell")] {
            let selector: TemporaryRuntimeSelector =
                TemporaryRuntimeSelector::new(Arc::new(started_mock()), Arc::new(untouched_mock()));
            start_with(&selector, runtime).await;
        }
    }

    #[tokio::test]
    async fn unknown_task_is_not_running() {
        let selector: TemporaryRuntimeSelector =
            TemporaryRuntimeSelector::new(Arc::new(untouched_mock()), Arc::new(untouched_mock()));
        let task_id: TaskId = TaskId::new();
        assert!(!selector.is_running(task_id).await);
        assert_eq!(
            selector.send(task_id, AgentCommand::Abort).await,
            Err(TaskError::AgentNotRunning)
        );
        assert_eq!(selector.stop(task_id).await, Ok(()));
    }
}
