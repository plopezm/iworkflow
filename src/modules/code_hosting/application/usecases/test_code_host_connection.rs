use std::sync::Arc;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_port::CodeHostClientPort;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;

/// Comprueba URL y token guardados; devuelve el usuario autenticado.
pub struct TestCodeHostConnection {
    repository: Arc<dyn CodeHostSettingsRepositoryPort>,
    client: Arc<dyn CodeHostClientPort>,
}

impl TestCodeHostConnection {
    pub fn new(
        repository: Arc<dyn CodeHostSettingsRepositoryPort>,
        client: Arc<dyn CodeHostClientPort>,
    ) -> Self {
        Self { repository, client }
    }

    pub async fn execute(&self) -> Result<String, CodeHostError> {
        let settings: CodeHostSettings = self
            .repository
            .get()
            .await?
            .ok_or(CodeHostError::NotConfigured)?;
        self.client.current_user(&settings).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::code_hosting::application::test_support;
    use crate::modules::code_hosting::mocks::{MockCodeHostClient, MockCodeHostSettingsRepository};

    #[tokio::test]
    async fn returns_authenticated_username() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_get()
            .returning(|| Ok(Some(test_support::settings())));
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_current_user()
            .returning(|_| Ok("pablo".to_string()));

        let user: String = TestCodeHostConnection::new(Arc::new(repository), Arc::new(client))
            .execute()
            .await
            .expect("user");

        assert_eq!(user, "pablo");
    }

    #[tokio::test]
    async fn fails_when_not_configured_without_calling_gitlab() {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository.expect_get().returning(|| Ok(None));
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client.expect_current_user().never();

        let result: Result<String, CodeHostError> =
            TestCodeHostConnection::new(Arc::new(repository), Arc::new(client))
                .execute()
                .await;

        assert_eq!(result, Err(CodeHostError::NotConfigured));
    }
}
