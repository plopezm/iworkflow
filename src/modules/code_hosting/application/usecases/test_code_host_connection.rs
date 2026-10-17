use std::sync::Arc;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_factory_port::CodeHostClientFactoryPort;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;

/// Comprueba URL y token guardados de un proveedor; devuelve el usuario autenticado.
pub struct TestCodeHostConnection {
    repository: Arc<dyn CodeHostSettingsRepositoryPort>,
    clients: Arc<dyn CodeHostClientFactoryPort>,
}

impl TestCodeHostConnection {
    pub fn new(
        repository: Arc<dyn CodeHostSettingsRepositoryPort>,
        clients: Arc<dyn CodeHostClientFactoryPort>,
    ) -> Self {
        Self {
            repository,
            clients,
        }
    }

    pub async fn execute(&self, provider: HostingProvider) -> Result<String, CodeHostError> {
        let settings: CodeHostSettings = self
            .repository
            .get(provider)
            .await?
            .ok_or(CodeHostError::NotConfigured(provider))?;
        self.clients
            .client_for(settings.provider())
            .current_user(&settings)
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::code_hosting::application::test_support;
    use crate::modules::code_hosting::domain::ports::outbound::code_host_client_port::CodeHostClientPort;
    use crate::modules::code_hosting::mocks::{
        MockCodeHostClient, MockCodeHostClientFactory, MockCodeHostSettingsRepository,
    };

    #[tokio::test]
    async fn returns_authenticated_username() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_get()
            .returning(|_| Ok(Some(test_support::settings())));
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_current_user()
            .returning(|_| Ok("pablo".to_string()));

        let user: String = TestCodeHostConnection::new(
            Arc::new(repository),
            Arc::new(test_support::factory_for(client)),
        )
        .execute(HostingProvider::GitLab)
        .await
        .expect("user");

        assert_eq!(user, "pablo");
    }

    #[tokio::test]
    async fn fails_when_not_configured_without_calling_the_provider() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository.expect_get().returning(|_| Ok(None));
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client.expect_current_user().never();

        let result: Result<String, CodeHostError> = TestCodeHostConnection::new(
            Arc::new(repository),
            Arc::new(test_support::factory_for(client)),
        )
        .execute(HostingProvider::GitHub)
        .await;

        assert_eq!(
            result,
            Err(CodeHostError::NotConfigured(HostingProvider::GitHub))
        );
    }

    #[tokio::test]
    async fn uses_the_client_of_the_requested_provider() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository.expect_get().returning(|_| {
            Ok(Some(
                CodeHostSettings::new(HostingProvider::GitHub, "https://github.com", "ghp", false)
                    .expect("settings"),
            ))
        });
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_current_user()
            .withf(|settings: &CodeHostSettings| settings.provider() == HostingProvider::GitHub)
            .returning(|_| Ok("octocat".to_string()));
        let shared: Arc<dyn CodeHostClientPort> = Arc::new(client);
        let mut factory: MockCodeHostClientFactory = MockCodeHostClientFactory::new();
        factory
            .expect_client_for()
            .withf(|provider: &HostingProvider| *provider == HostingProvider::GitHub)
            .times(1)
            .returning(move |_| shared.clone());

        let user: String = TestCodeHostConnection::new(Arc::new(repository), Arc::new(factory))
            .execute(HostingProvider::GitHub)
            .await
            .expect("user");

        assert_eq!(user, "octocat");
    }
}
