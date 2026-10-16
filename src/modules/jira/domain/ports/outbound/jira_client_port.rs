use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_comment::JiraComment;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::models::jira_issue::RemoteIssue;
use crate::modules::jira::domain::models::jira_transition::JiraTransition;

/// Jira Server/Data Center REST API v2.
#[async_trait::async_trait]
pub trait JiraClientPort: Send + Sync {
    /// Checks URL and token; returns the display name of the authenticated user.
    async fn verify(&self, connection: &JiraConnection) -> Result<String, JiraError>;
    async fn search(
        &self,
        connection: &JiraConnection,
        jql: &str,
    ) -> Result<Vec<RemoteIssue>, JiraError>;
    async fn get_issue(
        &self,
        connection: &JiraConnection,
        key: &str,
    ) -> Result<RemoteIssue, JiraError>;
    async fn comments(
        &self,
        connection: &JiraConnection,
        key: &str,
    ) -> Result<Vec<JiraComment>, JiraError>;
    async fn add_comment(
        &self,
        connection: &JiraConnection,
        key: &str,
        body: &str,
    ) -> Result<(), JiraError>;
    async fn transitions(
        &self,
        connection: &JiraConnection,
        key: &str,
    ) -> Result<Vec<JiraTransition>, JiraError>;
    async fn transition(
        &self,
        connection: &JiraConnection,
        key: &str,
        transition_id: &str,
    ) -> Result<(), JiraError>;
}
