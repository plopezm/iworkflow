use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::tasks::application::services::queued_agent_event_sink::{
    DEFAULT_QUEUE_CAPACITY, QueuedAgentEventSink,
};
use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::application::usecases::abort_task::AbortTask;
use crate::modules::tasks::application::usecases::add_task_dependency::AddTaskDependency;
use crate::modules::tasks::application::usecases::get_task_graph::GetTaskGraph;
use crate::modules::tasks::application::usecases::detach_task_from_graph::DetachTaskFromGraph;
use crate::modules::tasks::application::usecases::remove_task_dependency::RemoveTaskDependency;
use crate::modules::tasks::application::usecases::set_task_position::SetTaskPosition;
use crate::modules::tasks::application::usecases::trigger_successors::TriggerSuccessors;
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
use crate::modules::tasks::application::usecases::get_task_file::GetTaskFile;
use crate::modules::tasks::application::usecases::save_task_file::SaveTaskFile;
use crate::modules::tasks::application::usecases::get_task_subagents::GetTaskSubagents;
use crate::modules::tasks::application::usecases::handle_agent_event::HandleAgentEvent;
use crate::modules::tasks::application::usecases::list_done_tasks::ListDoneTasks;
use crate::modules::tasks::application::usecases::list_tasks::ListTasks;
use crate::modules::tasks::application::usecases::list_workspace_branches::ListWorkspaceBranches;
use crate::modules::tasks::application::usecases::list_workspace_options::ListWorkspaceOptions;
use crate::modules::tasks::application::usecases::merge_task::MergeTask;
use crate::modules::tasks::application::usecases::reconcile_tasks::ReconcileTasks;
use crate::modules::tasks::application::usecases::reset_task::ResetTask;
use crate::modules::tasks::application::usecases::resume_rate_limited_tasks::ResumeRateLimitedTasks;
use crate::modules::tasks::application::usecases::resume_task::ResumeTask;
use crate::modules::tasks::application::usecases::send_message::SendMessage;
use crate::modules::tasks::application::usecases::start_task::StartTask;
use crate::modules::tasks::application::usecases::update_task::UpdateTask;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::runtime_id::RuntimeId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::inbound::start_task_use_case::StartTaskUseCase;
use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;
use crate::modules::tasks::domain::ports::outbound::launch_settings_port::LaunchSettingsPort;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::task_event_publisher_port::TaskEventPublisherPort;
use crate::modules::tasks::domain::ports::outbound::task_file_port::TaskFilePort;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;
use crate::modules::tasks::infrastructure::adapters::outbound::events::broadcast_task_event_bus::BroadcastTaskEventBus;
use crate::modules::tasks::infrastructure::adapters::outbound::files::fs_task_file_adapter::FsTaskFileAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::git::caching_worktree::CachingWorktree;
use crate::modules::tasks::infrastructure::adapters::outbound::git::git_cli_worktree_adapter::GitCliWorktreeAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::retrying_task_repository::{
    RetryPolicy, RetryingTaskRepository,
};
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::task_attachment_repository_adapter::TaskAttachmentRepositoryAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::task_graph_repository_adapter::TaskGraphRepositoryAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::TaskRepositoryAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::storage::fs_attachment_storage_adapter::FsAttachmentStorageAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::agent_runtime_router::AgentRuntimeRouter;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::claude::claude_stream_runtime_adapter::ClaudeStreamRuntimeAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::cursor::cursor_acp_runtime_adapter::CursorAcpRuntimeAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::pi::pi_rpc_runtime_adapter::PiRpcRuntimeAdapter;

/// Id de gentle-shell (runtime predeterminado); igual a `AgentRuntime::GentleShell.as_str()`.
pub(crate) const GENTLE_SHELL_RUNTIME: &str = "gentle-shell";
/// Id de pi; igual a `AgentRuntime::Pi.as_str()`.
pub(crate) const PI_RUNTIME: &str = "pi";
/// Id de Claude Code; igual a `AgentRuntime::Claude.as_str()`.
pub(crate) const CLAUDE_RUNTIME: &str = "claude";
/// Id del agente de Cursor; igual a `AgentRuntime::Cursor.as_str()`.
pub(crate) const CURSOR_RUNTIME: &str = "cursor";

/// Configuración propia del feature de tareas.
#[derive(Debug, Clone)]
pub struct TasksSettings {
    pub gentle_shell_bin: String,
    pub pi_bin: String,
    pub claude_bin: String,
    pub cursor_bin: String,
    pub cursor_sessions_dir: PathBuf,
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
    pub resume_rate_limited_tasks: Arc<ResumeRateLimitedTasks>,
    pub merge_task: Arc<MergeTask>,
    pub discard_task: Arc<DiscardTask>,
    pub get_task: Arc<GetTask>,
    pub list_task_attachments: Arc<ListTaskAttachments>,
    pub get_task_attachment: Arc<GetTaskAttachment>,
    pub get_task_diff: Arc<GetTaskDiff>,
    pub get_task_events: Arc<GetTaskEvents>,
    pub get_task_file: Arc<GetTaskFile>,
    pub save_task_file: Arc<SaveTaskFile>,
    pub get_task_subagents: Arc<GetTaskSubagents>,
    pub list_tasks: Arc<ListTasks>,
    pub list_done_tasks: Arc<ListDoneTasks>,
    pub list_workspace_options: Arc<ListWorkspaceOptions>,
    pub list_workspace_branches: Arc<ListWorkspaceBranches>,
    pub reconcile_tasks: Arc<ReconcileTasks>,
    pub reset_task: Arc<ResetTask>,
    pub add_task_dependency: Arc<AddTaskDependency>,
    pub remove_task_dependency: Arc<RemoveTaskDependency>,
    pub detach_task_from_graph: Arc<DetachTaskFromGraph>,
    pub set_task_position: Arc<SetTaskPosition>,
    pub get_task_graph: Arc<GetTaskGraph>,
    pub trigger_successors: Arc<TriggerSuccessors>,
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
    ) -> Result<Self, TaskError> {
        let runtime: Arc<dyn AgentRuntimePort> = Arc::new(Self::build_router(
            settings.gentle_shell_bin,
            settings.pi_bin,
            settings.claude_bin,
            settings.cursor_bin,
            settings.cursor_sessions_dir,
        )?);
        Ok(Self::with_runtime(
            pool,
            settings.worktrees_dir,
            settings.attachments_dir,
            workspaces,
            runtime,
            launch_settings,
        ))
    }

    /// Registra gentle-shell (predeterminado), pi, Claude Code y Cursor.
    fn build_router(
        gentle_shell_bin: String,
        pi_bin: String,
        claude_bin: String,
        cursor_bin: String,
        cursor_sessions_dir: PathBuf,
    ) -> Result<AgentRuntimeRouter, TaskError> {
        let gentle_shell_id: RuntimeId = RuntimeId::parse(GENTLE_SHELL_RUNTIME)?;
        let pi_id: RuntimeId = RuntimeId::parse(PI_RUNTIME)?;
        let claude_id: RuntimeId = RuntimeId::parse(CLAUDE_RUNTIME)?;
        let cursor_id: RuntimeId = RuntimeId::parse(CURSOR_RUNTIME)?;
        let mut runtimes: HashMap<RuntimeId, Arc<dyn AgentRuntimePort>> = HashMap::new();
        runtimes.insert(
            gentle_shell_id.clone(),
            Arc::new(PiRpcRuntimeAdapter::new(gentle_shell_bin)),
        );
        runtimes.insert(pi_id, Arc::new(PiRpcRuntimeAdapter::new(pi_bin)));
        runtimes.insert(
            claude_id,
            Arc::new(ClaudeStreamRuntimeAdapter::new(claude_bin)),
        );
        runtimes.insert(
            cursor_id,
            Arc::new(CursorAcpRuntimeAdapter::new(
                cursor_bin,
                cursor_sessions_dir,
            )),
        );
        AgentRuntimeRouter::new(gentle_shell_id, runtimes)
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
        let graph: Arc<dyn TaskGraphRepositoryPort> =
            Arc::new(TaskGraphRepositoryAdapter::new(pool.clone()));
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
        let task_files: Arc<dyn TaskFilePort> = Arc::new(FsTaskFileAdapter::new());
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
        let starter: Arc<dyn StartTaskUseCase> = start_task.clone();
        let trigger_successors: Arc<TriggerSuccessors> = Arc::new(TriggerSuccessors::new(
            repository.clone(),
            graph.clone(),
            starter,
        ));
        let trigger: Arc<dyn SuccessorTrigger> = trigger_successors.clone();
        let send_message: Arc<SendMessage> =
            Arc::new(SendMessage::new(lifecycle.clone(), runtime.clone()));
        let resume_task: Arc<ResumeTask> = Arc::new(ResumeTask::new(
            lifecycle.clone(),
            worktrees.clone(),
            runtime.clone(),
            launch_settings,
            sink.clone(),
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
            send_message: send_message.clone(),
            abort_task: Arc::new(AbortTask::new(lifecycle.clone(), runtime.clone())),
            answer_dialog: Arc::new(AnswerDialog::new(lifecycle.clone(), runtime.clone())),
            finish_task: Arc::new(FinishTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime.clone(),
                Arc::new(AttachmentCleanup::new(attachments.clone(), storage.clone())),
            )),
            resume_rate_limited_tasks: Arc::new(ResumeRateLimitedTasks::new(
                lifecycle.clone(),
                runtime.clone(),
                resume_task.clone(),
                send_message,
            )),
            resume_task,
            merge_task: Arc::new(MergeTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                Some(trigger.clone()),
            )),
            discard_task: Arc::new(DiscardTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime.clone(),
            )),
            reset_task: Arc::new(ResetTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime.clone(),
            )),
            get_task_subagents: Arc::new(GetTaskSubagents::new(lifecycle.clone(), runtime)),
            get_task: Arc::new(GetTask::new(lifecycle.clone())),
            list_task_attachments: Arc::new(ListTaskAttachments::new(attachments.clone())),
            get_task_attachment: Arc::new(GetTaskAttachment::new(attachments, storage)),
            get_task_diff: Arc::new(GetTaskDiff::new(lifecycle.clone(), diff_worktrees)),
            get_task_events: Arc::new(GetTaskEvents::new(repository.clone())),
            get_task_file: Arc::new(GetTaskFile::new(lifecycle.clone(), task_files.clone())),
            // Plain (non-caching) worktree port: the commit must hit git directly.
            save_task_file: Arc::new(SaveTaskFile::new(
                lifecycle.clone(),
                task_files,
                worktrees.clone(),
            )),
            list_tasks: Arc::new(ListTasks::new(repository.clone())),
            list_done_tasks: Arc::new(ListDoneTasks::new(repository)),
            list_workspace_options: Arc::new(ListWorkspaceOptions::new(workspaces)),
            list_workspace_branches: Arc::new(ListWorkspaceBranches::new(
                lifecycle.clone(),
                worktrees.clone(),
            )),
            add_task_dependency: Arc::new(AddTaskDependency::new(
                lifecycle.clone(),
                graph.clone(),
                trigger.clone(),
            )),
            remove_task_dependency: Arc::new(RemoveTaskDependency::new(
                graph.clone(),
                trigger.clone(),
            )),
            detach_task_from_graph: Arc::new(DetachTaskFromGraph::new(graph.clone(), trigger)),
            set_task_position: Arc::new(SetTaskPosition::new(graph.clone())),
            get_task_graph: Arc::new(GetTaskGraph::new(graph)),
            trigger_successors,
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
    use std::path::Path;

    use super::*;
    use crate::core::db::pool::{build_pool, run_migrations};
    use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
    use crate::modules::tasks::domain::models::agent_command::MessageMode;
    use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
    use crate::modules::tasks::domain::models::task::TaskId;
    use crate::modules::tasks::mocks::{
        MockAgentEventSink, MockLaunchSettings, MockWorkspaceLookup,
    };

    // Binarios inexistentes: estos tests nunca llegan a lanzar un proceso.
    const SHELL_BIN: &str = "/nonexistent/iwf-test-gentle-shell";
    const PI_BIN: &str = "/nonexistent/iwf-test-pi";
    const CLAUDE_BIN: &str = "/nonexistent/iwf-test-claude";

    fn router() -> AgentRuntimeRouter {
        TasksContext::build_router(
            SHELL_BIN.to_string(),
            PI_BIN.to_string(),
            CLAUDE_BIN.to_string(),
            "agent".to_string(),
            PathBuf::from("/tmp/iwf-test-cursor-sessions"),
        )
        .expect("router")
    }

    async fn start_with(
        router: &AgentRuntimeRouter,
        runtime: Option<&str>,
    ) -> Result<(), TaskError> {
        let options: LaunchOptions = LaunchOptions {
            runtime: runtime.map(|r: &str| RuntimeId::parse(r).expect("runtime válido")),
            ..LaunchOptions::default()
        };
        let sink: Arc<dyn AgentEventSink> = Arc::new(MockAgentEventSink::new());
        router
            .start(TaskId::new(), Path::new("/tmp"), false, &options, sink)
            .await
    }

    #[test]
    fn registers_both_runtimes_with_full_capabilities() {
        let router: AgentRuntimeRouter = router();
        for raw in [GENTLE_SHELL_RUNTIME, PI_RUNTIME] {
            let id: RuntimeId = RuntimeId::parse(raw).expect("runtime válido");
            assert_eq!(
                router.runtime_capabilities(Some(&id)),
                AgentCapabilities::full()
            );
        }
    }

    #[test]
    fn claude_is_registered_with_its_own_capabilities() {
        let claude: RuntimeId = RuntimeId::parse(CLAUDE_RUNTIME).expect("runtime válido");
        let caps: AgentCapabilities = router().runtime_capabilities(Some(&claude));

        assert!(caps.dialogs && caps.resume && caps.model_selection && caps.thinking_levels);
        assert!(!caps.steer && !caps.follow_up && !caps.subagents);
    }

    #[test]
    fn cursor_is_registered_with_dialogs_resume_and_model_selection_only() {
        let cursor: RuntimeId = RuntimeId::parse(CURSOR_RUNTIME).expect("runtime válido");
        let caps: AgentCapabilities = router().runtime_capabilities(Some(&cursor));

        assert!(caps.dialogs && caps.resume && caps.model_selection);
        assert!(!caps.steer && !caps.follow_up && !caps.subagents && !caps.thinking_levels);
    }

    #[test]
    fn unknown_runtime_has_minimal_capabilities() {
        let codex: RuntimeId = RuntimeId::parse("codex").expect("runtime válido");
        assert_eq!(
            router().runtime_capabilities(Some(&codex)),
            AgentCapabilities::minimal()
        );
    }

    #[test]
    fn default_runtime_is_gentle_shell() {
        let gentle_shell: RuntimeId =
            RuntimeId::parse(GENTLE_SHELL_RUNTIME).expect("runtime válido");

        assert_eq!(router().default_runtime(), &gentle_shell);
        assert_eq!(
            router().runtime_capabilities(None),
            AgentCapabilities::full()
        );
    }

    #[tokio::test]
    async fn unknown_runtime_id_is_rejected_as_unsupported() {
        let result: Result<(), TaskError> = start_with(&router(), Some("codex")).await;

        assert!(matches!(
            result,
            Err(TaskError::Unsupported(message)) if message.contains("codex")
        ));
    }

    #[tokio::test(flavor = "multi_thread")]
    async fn new_wires_the_binary_path_against_a_migrated_sqlite() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let url: String = dir.path().join("test.db").to_string_lossy().into_owned();
        let pool: DbPool = build_pool(&url).expect("pool");
        run_migrations(&pool).await.expect("migraciones");
        let settings: TasksSettings = TasksSettings {
            gentle_shell_bin: SHELL_BIN.to_string(),
            pi_bin: PI_BIN.to_string(),
            claude_bin: "claude".to_string(),
            cursor_bin: "agent".to_string(),
            cursor_sessions_dir: dir.path().join("cursor-sessions"),
            worktrees_dir: dir.path().join("worktrees"),
            attachments_dir: dir.path().join("attachments"),
        };
        let workspaces: Arc<dyn WorkspaceLookupPort> = Arc::new(MockWorkspaceLookup::new());
        let launch_settings: Arc<dyn LaunchSettingsPort> = Arc::new(MockLaunchSettings::new());

        let context: TasksContext =
            TasksContext::new(pool, settings, workspaces, launch_settings).expect("contexto");

        let active: Vec<_> = context.list_tasks.execute(None).await.expect("lista");
        assert!(active.is_empty());
        let missing: Result<(), TaskError> = context
            .send_message
            .execute(
                TaskId::new(),
                "hola".to_string(),
                Vec::new(),
                MessageMode::Steer,
            )
            .await;
        assert!(matches!(missing, Err(TaskError::NotFound(_))));
    }
}
