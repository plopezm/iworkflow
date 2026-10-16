use std::sync::Arc;

use crate::modules::jira::application::usecases::get_connection::require_connection;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;

/// Posts a comment on the issue in Jira.
pub struct AddIssueComment {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
    client: Arc<dyn JiraClientPort>,
}

impl AddIssueComment {
    pub fn new(
        connections: Arc<dyn JiraConnectionRepositoryPort>,
        client: Arc<dyn JiraClientPort>,
    ) -> Self {
        Self {
            connections,
            client,
        }
    }

    pub async fn execute(&self, key: &str, body: &str) -> Result<(), JiraError> {
        let body: &str = body.trim();
        if body.is_empty() {
            return Err(JiraError::EmptyComment);
        }
        let connection = require_connection(&self.connections).await?;
        self.client.add_comment(&connection, key, body).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_connection::JiraConnection;
    use crate::modules::jira::mocks::{MockJiraClient, MockJiraConnectionRepository};

    #[tokio::test]
    async fn blank_comment_is_rejected_before_any_call() {
        let result = AddIssueComment::new(
            Arc::new(MockJiraConnectionRepository::new()),
            Arc::new(MockJiraClient::new()),
        )
        .execute("A-1", "  \n")
        .await;
        assert_eq!(result, Err(JiraError::EmptyComment));
    }

    #[tokio::test]
    async fn posts_the_trimmed_comment() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| {
            Ok(Some(
                JiraConnection::new("https://j", "t", false).expect("valid"),
            ))
        });
        let mut client = MockJiraClient::new();
        client
            .expect_add_comment()
            .withf(|_, key, body| key == "A-1" && body == "listo")
            .times(1)
            .returning(|_, _, _| Ok(()));
        AddIssueComment::new(Arc::new(connections), Arc::new(client))
            .execute("A-1", " listo ")
            .await
            .expect("posted");
    }
}
