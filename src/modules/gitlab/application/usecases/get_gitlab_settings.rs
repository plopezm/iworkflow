use std::sync::Arc;

use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use crate::modules::gitlab::domain::ports::outbound::gitlab_settings_repository_port::GitLabSettingsRepositoryPort;

/// Lee los ajustes de GitLab; `None` si aún no se han configurado.
pub struct GetGitLabSettings {
    repository: Arc<dyn GitLabSettingsRepositoryPort>,
}

impl GetGitLabSettings {
    pub fn new(repository: Arc<dyn GitLabSettingsRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<Option<GitLabSettings>, GitLabError> {
        self.repository.get().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::gitlab::application::test_support;
    use crate::modules::gitlab::mocks::MockGitLabSettingsRepository;

    #[tokio::test]
    async fn returns_stored_settings() {
        let mut repository: MockGitLabSettingsRepository = MockGitLabSettingsRepository::new();
        repository
            .expect_get()
            .returning(|| Ok(Some(test_support::settings())));

        let result: Option<GitLabSettings> = GetGitLabSettings::new(Arc::new(repository))
            .execute()
            .await
            .expect("get");

        assert_eq!(result, Some(test_support::settings()));
    }

    #[tokio::test]
    async fn returns_none_when_unconfigured() {
        let mut repository: MockGitLabSettingsRepository = MockGitLabSettingsRepository::new();
        repository.expect_get().returning(|| Ok(None));

        let result: Option<GitLabSettings> = GetGitLabSettings::new(Arc::new(repository))
            .execute()
            .await
            .expect("get");

        assert_eq!(result, None);
    }
}
