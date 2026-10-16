use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::priority_hint::PriorityHint;

/// What the tasks feature needs to create a task from a Jira issue.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NewLinkedTask {
    pub workspace_id: String,
    pub base_branch: Option<String>,
    pub title: String,
    pub prompt: String,
    pub priority: PriorityHint,
    pub start_now: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CreatedLinkedTask {
    pub task_id: String,
    /// Set when `start_now` was requested but the task stayed in the backlog.
    pub start_error: Option<String>,
}

#[async_trait::async_trait]
pub trait TaskCreationPort: Send + Sync {
    async fn create(&self, task: NewLinkedTask) -> Result<CreatedLinkedTask, JiraError>;
}
