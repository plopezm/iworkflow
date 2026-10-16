use std::sync::Arc;

use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use crate::modules::gitlab::domain::ports::outbound::gitlab_client_port::GitLabClientPort;
use crate::modules::gitlab::domain::ports::outbound::gitlab_settings_repository_port::GitLabSettingsRepositoryPort;

/// Comprueba URL y token guardados; devuelve el usuario autenticado.
pub struct TestGitLabConnection {
    repository: Arc<dyn GitLabSettingsRepositoryPort>,
    client: Arc<dyn GitLabClientPort>,
}

impl TestGitLabConnection {
    pub fn new(
        repository: Arc<dyn GitLabSettingsRepositoryPort>,
        client: Arc<dyn GitLabClientPort>,
    ) -> Self {
        Self { repository, client }
    }

    pub async fn execute(&self) -> Result<String, GitLabError> {
        let settings: GitLabSettings = self
            .repository
            .get()
            .await?
            .ok_or(GitLabError::NotConfigured)?;
        self.client.current_user(&settings).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::gitlab::application::test_support;
    use crate::modules::gitlab::mocks::{MockGitLabClient, MockGitLabSettingsRepository};

    #[tokio::test]
    async fn returns_authenticated_username() {
        let mut repository: MockGitLabSettingsRepository = MockGitLabSettingsRepository::new();
        repository
            .expect_get()
            .returning(|| Ok(Some(test_support::settings())));
        let mut client: MockGitLabClient = MockGitLabClient::new();
        client
            .expect_current_user()
            .returning(|_| Ok("pablo".to_string()));

        let user: String = TestGitLabConnection::new(Arc::new(repository), Arc::new(client))
            .execute()
            .await
            .expect("user");

        assert_eq!(user, "pablo");
    }

    #[tokio::test]
    async fn fails_when_not_configured_without_calling_gitlab() {
        let mut repository: MockGitLabSettingsRepository = MockGitLabSettingsRepository::new();
        repository.expect_get().returning(|| Ok(None));
        let mut client: MockGitLabClient = MockGitLabClient::new();
        client.expect_current_user().never();

        let result: Result<String, GitLabError> =
            TestGitLabConnection::new(Arc::new(repository), Arc::new(client))
                .execute()
                .await;

        assert_eq!(result, Err(GitLabError::NotConfigured));
    }
}
