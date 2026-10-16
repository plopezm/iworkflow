use std::sync::Arc;

use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use crate::modules::gitlab::domain::ports::outbound::gitlab_settings_repository_port::GitLabSettingsRepositoryPort;

#[derive(Clone, PartialEq, Eq)]
pub struct SaveGitLabSettingsInput {
    pub base_url: String,
    /// `None` o vacío conserva el token guardado (la UI nunca recibe el token).
    pub token: Option<String>,
    pub insecure_tls: bool,
}

// El token nunca debe acabar en logs.
impl std::fmt::Debug for SaveGitLabSettingsInput {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SaveGitLabSettingsInput")
            .field("base_url", &self.base_url)
            .field("token", &self.token.as_ref().map(|_| "***"))
            .field("insecure_tls", &self.insecure_tls)
            .finish()
    }
}

/// Valida y guarda la URL y el token de GitLab.
pub struct SaveGitLabSettings {
    repository: Arc<dyn GitLabSettingsRepositoryPort>,
}

impl SaveGitLabSettings {
    pub fn new(repository: Arc<dyn GitLabSettingsRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(
        &self,
        input: SaveGitLabSettingsInput,
    ) -> Result<GitLabSettings, GitLabError> {
        let provided: Option<String> = input
            .token
            .filter(|token: &String| !token.trim().is_empty());
        let token: String = match provided {
            Some(token) => token,
            None => self
                .repository
                .get()
                .await?
                .map(|stored: GitLabSettings| stored.token().to_string())
                .ok_or(GitLabError::MissingToken)?,
        };
        let settings: GitLabSettings =
            GitLabSettings::new(&input.base_url, &token, input.insecure_tls)?;
        self.repository.save(&settings).await?;
        Ok(settings)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::gitlab::application::test_support;
    use crate::modules::gitlab::mocks::MockGitLabSettingsRepository;

    fn input(base_url: &str, token: Option<&str>) -> SaveGitLabSettingsInput {
        SaveGitLabSettingsInput {
            base_url: base_url.to_string(),
            token: token.map(str::to_string),
            insecure_tls: true,
        }
    }

    #[tokio::test]
    async fn saves_normalized_settings() {
        let mut repository: MockGitLabSettingsRepository = MockGitLabSettingsRepository::new();
        repository
            .expect_save()
            .withf(|s: &GitLabSettings| {
                s.base_url() == "https://git.corp" && s.token() == "nuevo" && s.insecure_tls
            })
            .times(1)
            .returning(|_| Ok(()));

        let saved: GitLabSettings = SaveGitLabSettings::new(Arc::new(repository))
            .execute(input("https://git.corp/", Some(" nuevo ")))
            .await
            .expect("save");

        assert_eq!(saved.token(), "nuevo");
    }

    #[tokio::test]
    async fn blank_token_keeps_the_stored_one() {
        let mut repository: MockGitLabSettingsRepository = MockGitLabSettingsRepository::new();
        repository
            .expect_get()
            .returning(|| Ok(Some(test_support::settings())));
        repository
            .expect_save()
            .withf(|s: &GitLabSettings| s.token() == "tok" && s.base_url() == "https://otra.corp")
            .times(1)
            .returning(|_| Ok(()));

        SaveGitLabSettings::new(Arc::new(repository))
            .execute(input("https://otra.corp", Some("   ")))
            .await
            .expect("save");
    }

    #[tokio::test]
    async fn missing_token_without_stored_one_is_rejected_and_nothing_saved() {
        let mut repository: MockGitLabSettingsRepository = MockGitLabSettingsRepository::new();
        repository.expect_get().returning(|| Ok(None));
        repository.expect_save().never();

        let result: Result<GitLabSettings, GitLabError> =
            SaveGitLabSettings::new(Arc::new(repository))
                .execute(input("https://git.corp", None))
                .await;

        assert_eq!(result, Err(GitLabError::MissingToken));
    }

    #[tokio::test]
    async fn invalid_url_is_rejected_and_nothing_saved() {
        let mut repository: MockGitLabSettingsRepository = MockGitLabSettingsRepository::new();
        repository.expect_save().never();

        let result: Result<GitLabSettings, GitLabError> =
            SaveGitLabSettings::new(Arc::new(repository))
                .execute(input("git.corp", Some("tok")))
                .await;

        assert!(matches!(result, Err(GitLabError::InvalidUrl(_))));
    }
}
