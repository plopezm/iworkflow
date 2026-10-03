use std::path::PathBuf;
use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::application::usecases::abort_task::AbortTask;
use crate::modules::tasks::application::usecases::answer_dialog::AnswerDialog;
use crate::modules::tasks::application::usecases::create_task::CreateTask;
use crate::modules::tasks::application::usecases::delete_task::DeleteTask;
use crate::modules::tasks::application::usecases::delete_workspace_tasks::DeleteWorkspaceTasks;
use crate::modules::tasks::application::usecases::discard_task::DiscardTask;
use crate::modules::tasks::application::usecases::finish_task::FinishTask;
use crate::modules::tasks::application::usecases::get_task::GetTask;
use crate::modules::tasks::application::usecases::get_task_diff::GetTaskDiff;
use crate::modules::tasks::application::usecases::get_task_events::GetTaskEvents;
use crate::modules::tasks::application::usecases::handle_agent_event::HandleAgentEvent;
use crate::modules::tasks::application::usecases::list_tasks::ListTasks;
use crate::modules::tasks::application::usecases::list_workspace_options::ListWorkspaceOptions;
use crate::modules::tasks::application::usecases::merge_task::MergeTask;
use crate::modules::tasks::application::usecases::reconcile_tasks::ReconcileTasks;
use crate::modules::tasks::application::usecases::resume_task::ResumeTask;
use crate::modules::tasks::application::usecases::send_message::SendMessage;
use crate::modules::tasks::application::usecases::start_task::StartTask;
use crate::modules::tasks::application::usecases::update_task::UpdateTask;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::launch_settings_port::LaunchSettingsPort;
use crate::modules::tasks::domain::ports::outbound::task_event_publisher_port::TaskEventPublisherPort;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;
use crate::modules::tasks::infrastructure::adapters::outbound::events::broadcast_task_event_bus::BroadcastTaskEventBus;
use crate::modules::tasks::infrastructure::adapters::outbound::git::git_cli_worktree_adapter::GitCliWorktreeAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::TaskRepositoryAdapter;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::gentle_shell_runtime_adapter::GentleShellRuntimeAdapter;

/// Configuración propia del feature de tareas.
#[derive(Debug, Clone)]
pub struct TasksSettings {
    pub gentle_shell_bin: String,
    pub pi_bin: String,
    pub worktrees_dir: PathBuf,
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
    pub get_task_diff: Arc<GetTaskDiff>,
    pub get_task_events: Arc<GetTaskEvents>,
    pub list_tasks: Arc<ListTasks>,
    pub list_workspace_options: Arc<ListWorkspaceOptions>,
    pub reconcile_tasks: Arc<ReconcileTasks>,
    /// Bus en vivo consumido por el endpoint SSE.
    pub event_bus: BroadcastTaskEventBus,
}

impl TasksContext {
    pub fn new(
        pool: DbPool,
        settings: TasksSettings,
        workspaces: Arc<dyn WorkspaceLookupPort>,
        launch_settings: Arc<dyn LaunchSettingsPort>,
    ) -> Self {
        let runtime: Arc<dyn AgentRuntimePort> = Arc::new(GentleShellRuntimeAdapter::new(
            settings.gentle_shell_bin,
            settings.pi_bin,
        ));
        Self::with_runtime(
            pool,
            settings.worktrees_dir,
            workspaces,
            runtime,
            launch_settings,
        )
    }

    /// Permite sustituir el runtime de agentes (tests de extremo a extremo).
    pub fn with_runtime(
        pool: DbPool,
        worktrees_dir: PathBuf,
        workspaces: Arc<dyn WorkspaceLookupPort>,
        runtime: Arc<dyn AgentRuntimePort>,
        launch_settings: Arc<dyn LaunchSettingsPort>,
    ) -> Self {
        let repository: Arc<dyn TaskRepositoryPort> = Arc::new(TaskRepositoryAdapter::new(pool));
        let event_bus: BroadcastTaskEventBus = BroadcastTaskEventBus::new();
        let publisher: Arc<dyn TaskEventPublisherPort> = Arc::new(event_bus.clone());
        let worktrees: Arc<dyn WorktreePort> = Arc::new(GitCliWorktreeAdapter::new(worktrees_dir));
        let lifecycle: Arc<TaskLifecycle> = Arc::new(TaskLifecycle::new(
            repository.clone(),
            publisher,
            workspaces.clone(),
        ));
        let sink: Arc<dyn AgentEventSink> = Arc::new(HandleAgentEvent::new(lifecycle.clone()));
        let start_task: Arc<StartTask> = Arc::new(StartTask::new(
            lifecycle.clone(),
            worktrees.clone(),
            runtime.clone(),
            launch_settings.clone(),
            sink.clone(),
        ));

        Self {
            create_task: Arc::new(CreateTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                start_task.clone(),
            )),
            start_task,
            update_task: Arc::new(UpdateTask::new(lifecycle.clone())),
            delete_task: Arc::new(DeleteTask::new(lifecycle.clone())),
            delete_workspace_tasks: Arc::new(DeleteWorkspaceTasks::new(lifecycle.clone())),
            send_message: Arc::new(SendMessage::new(lifecycle.clone(), runtime.clone())),
            abort_task: Arc::new(AbortTask::new(lifecycle.clone(), runtime.clone())),
            answer_dialog: Arc::new(AnswerDialog::new(lifecycle.clone(), runtime.clone())),
            finish_task: Arc::new(FinishTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime.clone(),
            )),
            resume_task: Arc::new(ResumeTask::new(
                lifecycle.clone(),
                runtime.clone(),
                launch_settings,
                sink,
            )),
            merge_task: Arc::new(MergeTask::new(lifecycle.clone(), worktrees.clone())),
            discard_task: Arc::new(DiscardTask::new(
                lifecycle.clone(),
                worktrees.clone(),
                runtime,
            )),
            get_task: Arc::new(GetTask::new(lifecycle.clone())),
            get_task_diff: Arc::new(GetTaskDiff::new(lifecycle.clone(), worktrees)),
            get_task_events: Arc::new(GetTaskEvents::new(repository.clone())),
            list_tasks: Arc::new(ListTasks::new(repository)),
            list_workspace_options: Arc::new(ListWorkspaceOptions::new(workspaces)),
            reconcile_tasks: Arc::new(ReconcileTasks::new(lifecycle)),
            event_bus,
        }
    }
}
