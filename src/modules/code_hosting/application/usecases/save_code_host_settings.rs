use std::sync::Arc;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;

#[derive(Clone, PartialEq, Eq)]
pub struct SaveCodeHostSettingsInput {
    pub provider: HostingProvider,
    pub base_url: String,
    /// `None` o vacío conserva el token guardado (la UI nunca recibe el token).
    pub token: Option<String>,
    pub insecure_tls: bool,
}

// El token nunca debe acabar en logs.
impl std::fmt::Debug for SaveCodeHostSettingsInput {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SaveCodeHostSettingsInput")
            .field("provider", &self.provider)
            .field("base_url", &self.base_url)
            .field("token", &self.token.as_ref().map(|_| "***"))
            .field("insecure_tls", &self.insecure_tls)
            .finish()
    }
}

/// Valida y guarda la URL y el token de un proveedor.
pub struct SaveCodeHostSettings {
    repository: Arc<dyn CodeHostSettingsRepositoryPort>,
}

impl SaveCodeHostSettings {
    pub fn new(repository: Arc<dyn CodeHostSettingsRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(
        &self,
        input: SaveCodeHostSettingsInput,
    ) -> Result<CodeHostSettings, CodeHostError> {
        let provided: Option<String> = input
            .token
            .filter(|token: &String| !token.trim().is_empty());
        let settings: CodeHostSettings = match provided {
            Some(token) => {
                CodeHostSettings::new(input.provider, &input.base_url, &token, input.insecure_tls)?
            }
            None => {
                let stored: CodeHostSettings = self
                    .repository
                    .get(input.provider)
                    .await?
                    .ok_or(CodeHostError::MissingToken)?;
                let reused: CodeHostSettings = CodeHostSettings::new(
                    input.provider,
                    &input.base_url,
                    stored.token(),
                    input.insecure_tls,
                )?;
                // El token guardado no se reenvía a otro servidor: hay que volver a introducirlo.
                if reused.base_url() != stored.base_url() {
                    return Err(CodeHostError::TokenRequiredForNewUrl);
                }
                reused
            }
        };
        self.repository.save(&settings).await?;
        Ok(settings)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::code_hosting::application::test_support;
    use crate::modules::code_hosting::mocks::MockCodeHostSettingsRepository;

    fn input(base_url: &str, token: Option<&str>) -> SaveCodeHostSettingsInput {
        SaveCodeHostSettingsInput {
            provider: HostingProvider::GitLab,
            base_url: base_url.to_string(),
            token: token.map(str::to_string),
            insecure_tls: true,
        }
    }

    #[tokio::test]
    async fn saves_normalized_settings() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_save()
            .withf(|s: &CodeHostSettings| {
                s.base_url() == "https://git.corp" && s.token() == "nuevo" && s.insecure_tls
            })
            .times(1)
            .returning(|_| Ok(()));

        let saved: CodeHostSettings = SaveCodeHostSettings::new(Arc::new(repository))
            .execute(input("https://git.corp/", Some(" nuevo ")))
            .await
            .expect("save");

        assert_eq!(saved.token(), "nuevo");
    }

    #[tokio::test]
    async fn blank_token_keeps_the_stored_one_for_the_same_url() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_get()
            .returning(|_| Ok(Some(test_support::settings())));
        repository
            .expect_save()
            .withf(|s: &CodeHostSettings| s.token() == "tok" && s.base_url() == "https://git.corp")
            .times(1)
            .returning(|_| Ok(()));

        SaveCodeHostSettings::new(Arc::new(repository))
            .execute(input("https://git.corp/", Some("   ")))
            .await
            .expect("save");
    }

    #[tokio::test]
    async fn blank_token_with_a_different_url_is_rejected_and_nothing_saved() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_get()
            .returning(|_| Ok(Some(test_support::settings())));
        repository.expect_save().never();

        let result: Result<CodeHostSettings, CodeHostError> =
            SaveCodeHostSettings::new(Arc::new(repository))
                .execute(input("https://otra.corp", None))
                .await;

        assert_eq!(result, Err(CodeHostError::TokenRequiredForNewUrl));
    }

    #[tokio::test]
    async fn missing_token_without_stored_one_is_rejected_and_nothing_saved() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository.expect_get().returning(|_| Ok(None));
        repository.expect_save().never();

        let result: Result<CodeHostSettings, CodeHostError> =
            SaveCodeHostSettings::new(Arc::new(repository))
                .execute(input("https://git.corp", None))
                .await;

        assert_eq!(result, Err(CodeHostError::MissingToken));
    }

    #[tokio::test]
    async fn invalid_url_is_rejected_and_nothing_saved() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository.expect_save().never();

        let result: Result<CodeHostSettings, CodeHostError> =
            SaveCodeHostSettings::new(Arc::new(repository))
                .execute(input("git.corp", Some("tok")))
                .await;

        assert!(matches!(result, Err(CodeHostError::InvalidUrl(_))));
    }

    #[tokio::test]
    async fn saves_each_provider_under_its_own_identity() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_save()
            .withf(|s: &CodeHostSettings| {
                s.provider() == HostingProvider::GitHub && s.base_url() == "https://github.com"
            })
            .times(1)
            .returning(|_| Ok(()));
        let mut github: SaveCodeHostSettingsInput = input("https://github.com", Some("ghp_x"));
        github.provider = HostingProvider::GitHub;

        let saved: CodeHostSettings = SaveCodeHostSettings::new(Arc::new(repository))
            .execute(github)
            .await
            .expect("save");

        assert_eq!(saved.provider(), HostingProvider::GitHub);
    }

    #[tokio::test]
    async fn blank_token_only_reuses_the_stored_token_of_the_same_provider() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_get()
            .withf(|provider: &HostingProvider| *provider == HostingProvider::GitHub)
            .returning(|_| Ok(None));
        repository.expect_save().never();
        let mut github: SaveCodeHostSettingsInput = input("https://github.com", None);
        github.provider = HostingProvider::GitHub;

        let result: Result<CodeHostSettings, CodeHostError> =
            SaveCodeHostSettings::new(Arc::new(repository))
                .execute(github)
                .await;

        assert_eq!(result, Err(CodeHostError::MissingToken));
    }
}
