use std::sync::Arc;

use crate::modules::jira::application::usecases::get_connection::require_connection;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_comment::JiraComment;
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;

/// Comments are read live from Jira; they are not imported.
pub struct ListIssueComments {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
    client: Arc<dyn JiraClientPort>,
}

impl ListIssueComments {
    pub fn new(
        connections: Arc<dyn JiraConnectionRepositoryPort>,
        client: Arc<dyn JiraClientPort>,
    ) -> Self {
        Self {
            connections,
            client,
        }
    }

    pub async fn execute(&self, key: &str) -> Result<Vec<JiraComment>, JiraError> {
        let connection = require_connection(&self.connections).await?;
        self.client.comments(&connection, key).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_connection::JiraConnection;
    use crate::modules::jira::mocks::{MockJiraClient, MockJiraConnectionRepository};
    use chrono::Utc;

    #[tokio::test]
    async fn reads_comments_of_the_issue() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| {
            Ok(Some(
                JiraConnection::new("https://j", "t", "", false).expect("valid"),
            ))
        });
        let mut client = MockJiraClient::new();
        client
            .expect_comments()
            .withf(|_, key| key == "A-1")
            .returning(|_, _| {
                Ok(vec![JiraComment {
                    id: "1".to_string(),
                    author: "Ana".to_string(),
                    body: "hola".to_string(),
                    created_at: Utc::now(),
                }])
            });
        let comments = ListIssueComments::new(Arc::new(connections), Arc::new(client))
            .execute("A-1")
            .await
            .expect("comments");
        assert_eq!(comments[0].body, "hola");
    }
}
