use std::path::{Path, PathBuf};
use std::sync::Arc;

use mockall::mock;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::models::task_priority::TaskPriority;
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::models::worktree::{MergeOutcome, TaskDiff};
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::domain::ports::outbound::launch_settings_port::LaunchSettingsPort;
use crate::modules::tasks::domain::ports::outbound::task_event_publisher_port::TaskEventPublisherPort;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

mock! {
    pub TaskRepository {}

    #[async_trait::async_trait]
    impl TaskRepositoryPort for TaskRepository {
        async fn insert(&self, task: &Task) -> Result<(), TaskError>;
        async fn find(&self, id: TaskId) -> Result<Option<Task>, TaskError>;
        async fn list(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError>;
        async fn list_by_status(&self, statuses: &[TaskStatus]) -> Result<Vec<Task>, TaskError>;
        async fn update_status(
            &self,
            id: TaskId,
            status: TaskStatus,
            error: Option<String>,
        ) -> Result<(), TaskError>;
        async fn update_details_if_backlog(&self, id: TaskId, title: String, prompt: String, priority: TaskPriority) -> Result<bool, TaskError>;
        async fn claim_for_start(
            &self,
            id: TaskId,
            branch: String,
            worktree_path: PathBuf,
            base_branch: String,
        ) -> Result<bool, TaskError>;
        async fn revert_start_claim(&self, id: TaskId) -> Result<bool, TaskError>;
        async fn add_pending_dialog(&self, id: TaskId, dialog: PendingDialog) -> Result<(), TaskError>;
        async fn remove_pending_dialog(&self, id: TaskId, dialog_id: String) -> Result<(), TaskError>;
        async fn clear_pending_dialogs(&self, id: TaskId) -> Result<(), TaskError>;
        async fn set_subagent_activity(&self, id: TaskId, activity: SubagentActivity) -> Result<(), TaskError>;
        async fn delete_if_backlog(&self, id: TaskId) -> Result<bool, TaskError>;
        async fn delete_inactive_by_workspace(&self, workspace_id: String) -> Result<Option<Vec<TaskId>>, TaskError>;
        async fn append_event(&self, id: TaskId, kind: TaskEventKind) -> Result<TaskEvent, TaskError>;
        async fn events(&self, id: TaskId, after_seq: Option<i64>) -> Result<Vec<TaskEvent>, TaskError>;
    }
}

mock! {
    pub Worktree {}

    #[async_trait::async_trait]
    impl WorktreePort for Worktree {
        fn worktree_path(&self, workspace_name: &str, dir_name: &str) -> PathBuf;
        async fn create(&self, repo: &Path, base_branch: &str, branch: &str, path: &Path) -> Result<(), TaskError>;
        async fn commit_all(&self, path: &Path, message: &str) -> Result<bool, TaskError>;
        async fn diff(&self, repo: &Path, base_branch: &str, branch: &str) -> Result<TaskDiff, TaskError>;
        async fn merge(&self, repo: &Path, base_branch: &str, branch: &str, message: &str) -> Result<MergeOutcome, TaskError>;
        async fn remove(&self, repo: &Path, path: &Path, branch: &str) -> Result<(), TaskError>;
    }
}

mock! {
    pub AgentRuntime {}

    #[async_trait::async_trait]
    impl AgentRuntimePort for AgentRuntime {
        async fn start(
            &self,
            task_id: TaskId,
            cwd: &Path,
            resume: bool,
            options: &LaunchOptions,
            sink: Arc<dyn AgentEventSink>,
        ) -> Result<(), TaskError>;
        async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError>;
        async fn stop(&self, task_id: TaskId) -> Result<(), TaskError>;
        async fn is_running(&self, task_id: TaskId) -> bool;
    }
}

mock! {
    pub TaskEventPublisher {}

    impl TaskEventPublisherPort for TaskEventPublisher {
        fn publish(&self, event: TaskEvent);
    }
}

mock! {
    pub WorkspaceLookup {}

    #[async_trait::async_trait]
    impl WorkspaceLookupPort for WorkspaceLookup {
        async fn find(&self, workspace_id: &str) -> Result<Option<WorkspaceRef>, TaskError>;
        async fn list(&self) -> Result<Vec<WorkspaceRef>, TaskError>;
    }
}

mock! {
    pub LaunchSettings {}

    #[async_trait::async_trait]
    impl LaunchSettingsPort for LaunchSettings {
        async fn prepare(&self, cwd: &Path) -> Result<LaunchOptions, TaskError>;
    }
}

mock! {
    pub AgentEventSink {}

    #[async_trait::async_trait]
    impl AgentEventSink for AgentEventSink {
        async fn handle(&self, task_id: TaskId, event: AgentEvent);
    }
}
