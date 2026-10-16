use std::collections::HashMap;

use chrono::{DateTime, Utc};
use mockall::mock;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
use crate::modules::jira::domain::models::jira_comment::JiraComment;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::models::jira_issue::{JiraIssue, RemoteIssue};
use crate::modules::jira::domain::models::jira_lane::{JiraLane, LaneBatch, LaneDefinition};
use crate::modules::jira::domain::models::jira_transition::JiraTransition;
use crate::modules::jira::domain::models::workspace_option::{BranchOption, WorkspaceOption};
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;
use crate::modules::jira::domain::ports::outbound::task_creation_port::{
    CreatedLinkedTask, NewLinkedTask, TaskCreationPort,
};
use crate::modules::jira::domain::ports::outbound::workspace_catalog_port::WorkspaceCatalogPort;

mock! {
    pub JiraConnectionRepository {}

    #[async_trait::async_trait]
    impl JiraConnectionRepositoryPort for JiraConnectionRepository {
        async fn get(&self) -> Result<Option<JiraConnection>, JiraError>;
        async fn save(&self, connection: &JiraConnection) -> Result<(), JiraError>;
        async fn delete(&self) -> Result<(), JiraError>;
        async fn record_sync(&self, at: DateTime<Utc>, error: Option<String>) -> Result<(), JiraError>;
    }
}

mock! {
    pub JiraIssueRepository {}

    #[async_trait::async_trait]
    impl JiraIssueRepositoryPort for JiraIssueRepository {
        async fn list(&self) -> Result<Vec<JiraIssue>, JiraError>;
        async fn find(&self, key: &str) -> Result<Option<JiraIssue>, JiraError>;
        async fn lane_memberships(&self) -> Result<HashMap<String, Vec<String>>, JiraError>;
        async fn replace_synced(&self, batches: &[LaneBatch], synced_at: DateTime<Utc>) -> Result<usize, JiraError>;
        async fn update_remote(&self, issue: &RemoteIssue, at: DateTime<Utc>) -> Result<(), JiraError>;
        async fn set_local_state(&self, key: &str, state: IssueLocalState, task_id: Option<String>) -> Result<(), JiraError>;
        async fn delete_all(&self) -> Result<(), JiraError>;
    }
}

mock! {
    pub JiraLaneRepository {}

    #[async_trait::async_trait]
    impl JiraLaneRepositoryPort for JiraLaneRepository {
        async fn list(&self) -> Result<Vec<JiraLane>, JiraError>;
        async fn find(&self, id: &str) -> Result<Option<JiraLane>, JiraError>;
        async fn create(&self, definition: &LaneDefinition) -> Result<JiraLane, JiraError>;
        async fn update(&self, id: &str, definition: &LaneDefinition) -> Result<JiraLane, JiraError>;
        async fn delete(&self, id: &str) -> Result<(), JiraError>;
        async fn set_collapsed(&self, id: &str, collapsed: bool) -> Result<(), JiraError>;
    }
}

mock! {
    pub JiraClient {}

    #[async_trait::async_trait]
    impl JiraClientPort for JiraClient {
        async fn verify(&self, connection: &JiraConnection) -> Result<String, JiraError>;
        async fn search(&self, connection: &JiraConnection, jql: &str) -> Result<Vec<RemoteIssue>, JiraError>;
        async fn get_issue(&self, connection: &JiraConnection, key: &str) -> Result<RemoteIssue, JiraError>;
        async fn comments(&self, connection: &JiraConnection, key: &str) -> Result<Vec<JiraComment>, JiraError>;
        async fn add_comment(&self, connection: &JiraConnection, key: &str, body: &str) -> Result<(), JiraError>;
        async fn transitions(&self, connection: &JiraConnection, key: &str) -> Result<Vec<JiraTransition>, JiraError>;
        async fn transition(&self, connection: &JiraConnection, key: &str, transition_id: &str) -> Result<(), JiraError>;
    }
}

mock! {
    pub TaskCreation {}

    #[async_trait::async_trait]
    impl TaskCreationPort for TaskCreation {
        async fn create(&self, task: NewLinkedTask) -> Result<CreatedLinkedTask, JiraError>;
    }
}

mock! {
    pub WorkspaceCatalog {}

    #[async_trait::async_trait]
    impl WorkspaceCatalogPort for WorkspaceCatalog {
        async fn workspaces(&self) -> Result<Vec<WorkspaceOption>, JiraError>;
        async fn branches(&self, workspace_id: &str) -> Result<Vec<BranchOption>, JiraError>;
    }
}
