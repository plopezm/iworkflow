use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;

pub struct SaveConnectionInput {
    pub base_url: String,
    /// Blank keeps the stored token, so the token never has to travel back to the browser.
    pub token: String,
    pub jql: String,
    pub accept_invalid_certs: bool,
}

/// Validates the connection against the server and only then stores it. Pointing the connection
/// at another server discards the issues imported from the previous one.
pub struct SaveConnection {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
    issues: Arc<dyn JiraIssueRepositoryPort>,
    client: Arc<dyn JiraClientPort>,
}

impl SaveConnection {
    pub fn new(
        connections: Arc<dyn JiraConnectionRepositoryPort>,
        issues: Arc<dyn JiraIssueRepositoryPort>,
        client: Arc<dyn JiraClientPort>,
    ) -> Self {
        Self {
            connections,
            issues,
            client,
        }
    }

    pub async fn execute(&self, input: SaveConnectionInput) -> Result<JiraConnection, JiraError> {
        let existing: Option<JiraConnection> = self.connections.get().await?;
        let token: String = match input.token.trim() {
            "" => existing
                .as_ref()
                .map(|c: &JiraConnection| c.token.clone())
                .unwrap_or_default(),
            provided => provided.to_string(),
        };
        let connection: JiraConnection = JiraConnection::new(
            &input.base_url,
            &token,
            &input.jql,
            input.accept_invalid_certs,
        )?;
        self.client.verify(&connection).await?;
        if existing
            .as_ref()
            .is_some_and(|c: &JiraConnection| c.base_url != connection.base_url)
        {
            self.issues.delete_all().await?;
        }
        self.connections.save(&connection).await?;
        self.connections
            .get()
            .await?
            .ok_or(JiraError::NotConfigured)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::mocks::{
        MockJiraClient, MockJiraConnectionRepository, MockJiraIssueRepository,
    };

    fn input(token: &str) -> SaveConnectionInput {
        SaveConnectionInput {
            base_url: "https://jira.local/".to_string(),
            token: token.to_string(),
            jql: String::new(),
            accept_invalid_certs: false,
        }
    }

    fn stored(url: &str, token: &str) -> JiraConnection {
        JiraConnection::new(url, token, "", false).expect("valid")
    }

    #[tokio::test]
    async fn verifies_then_saves_a_new_connection() {
        let mut connections = MockJiraConnectionRepository::new();
        let mut seq = mockall::Sequence::new();
        connections
            .expect_get()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|| Ok(None));
        connections
            .expect_save()
            .withf(|c: &JiraConnection| c.base_url == "https://jira.local" && c.token == "tok")
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_| Ok(()));
        connections
            .expect_get()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|| Ok(Some(stored("https://jira.local", "tok"))));
        let mut client = MockJiraClient::new();
        client
            .expect_verify()
            .times(1)
            .returning(|_| Ok("Ana".to_string()));
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_delete_all().never();

        let saved: JiraConnection =
            SaveConnection::new(Arc::new(connections), Arc::new(issues), Arc::new(client))
                .execute(input("tok"))
                .await
                .expect("saved");
        assert_eq!(saved.token, "tok");
    }

    #[tokio::test]
    async fn a_failed_verification_saves_nothing() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| Ok(None));
        connections.expect_save().never();
        let mut client = MockJiraClient::new();
        client
            .expect_verify()
            .returning(|_| Err(JiraError::Unauthorized));

        let result = SaveConnection::new(
            Arc::new(connections),
            Arc::new(MockJiraIssueRepository::new()),
            Arc::new(client),
        )
        .execute(input("bad"))
        .await;
        assert_eq!(result, Err(JiraError::Unauthorized));
    }

    #[tokio::test]
    async fn blank_token_keeps_the_stored_one() {
        let mut connections = MockJiraConnectionRepository::new();
        connections
            .expect_get()
            .returning(|| Ok(Some(stored("https://jira.local", "kept"))));
        connections
            .expect_save()
            .withf(|c: &JiraConnection| c.token == "kept")
            .times(1)
            .returning(|_| Ok(()));
        let mut client = MockJiraClient::new();
        client
            .expect_verify()
            .withf(|c| c.token == "kept")
            .returning(|_| Ok(String::new()));
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_delete_all().never();

        SaveConnection::new(Arc::new(connections), Arc::new(issues), Arc::new(client))
            .execute(input("  "))
            .await
            .expect("saved");
    }

    #[tokio::test]
    async fn blank_token_without_stored_connection_is_rejected() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| Ok(None));
        let result = SaveConnection::new(
            Arc::new(connections),
            Arc::new(MockJiraIssueRepository::new()),
            Arc::new(MockJiraClient::new()),
        )
        .execute(input(""))
        .await;
        assert_eq!(result, Err(JiraError::EmptyToken));
    }

    #[tokio::test]
    async fn changing_the_server_discards_imported_issues() {
        let mut connections = MockJiraConnectionRepository::new();
        connections
            .expect_get()
            .returning(|| Ok(Some(stored("https://old.local", "tok"))));
        connections.expect_save().times(1).returning(|_| Ok(()));
        let mut client = MockJiraClient::new();
        client.expect_verify().returning(|_| Ok(String::new()));
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_delete_all().times(1).returning(|| Ok(()));

        SaveConnection::new(Arc::new(connections), Arc::new(issues), Arc::new(client))
            .execute(input("tok"))
            .await
            .expect("saved");
    }
}
