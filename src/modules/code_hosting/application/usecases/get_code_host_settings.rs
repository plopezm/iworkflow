use std::sync::Arc;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;

/// Lee los ajustes de un proveedor; `None` si aún no se han configurado.
pub struct GetCodeHostSettings {
    repository: Arc<dyn CodeHostSettingsRepositoryPort>,
}

impl GetCodeHostSettings {
    pub fn new(repository: Arc<dyn CodeHostSettingsRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(
        &self,
        provider: HostingProvider,
    ) -> Result<Option<CodeHostSettings>, CodeHostError> {
        self.repository.get(provider).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::code_hosting::application::test_support;
    use crate::modules::code_hosting::mocks::MockCodeHostSettingsRepository;

    #[tokio::test]
    async fn returns_stored_settings() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_get()
            .withf(|provider: &HostingProvider| *provider == HostingProvider::GitLab)
            .returning(|_| Ok(Some(test_support::settings())));

        let result: Option<CodeHostSettings> = GetCodeHostSettings::new(Arc::new(repository))
            .execute(HostingProvider::GitLab)
            .await
            .expect("get");

        assert_eq!(result, Some(test_support::settings()));
    }

    #[tokio::test]
    async fn returns_none_when_unconfigured() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository.expect_get().returning(|_| Ok(None));

        let result: Option<CodeHostSettings> = GetCodeHostSettings::new(Arc::new(repository))
            .execute(HostingProvider::GitLab)
            .await
            .expect("get");

        assert_eq!(result, None);
    }
}
