use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Reads whether the user opted in to serving the app on every network interface.
pub struct GetNetworkAccess {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl GetNetworkAccess {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<bool, PreferencesError> {
        self.repository.network_access().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::preferences::mocks::MockPreferencesRepository;

    #[tokio::test]
    async fn returns_the_stored_choice() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository.expect_network_access().returning(|| Ok(true));

        let enabled: bool = GetNetworkAccess::new(Arc::new(repository))
            .execute()
            .await
            .expect("get");

        assert!(enabled);
    }

    #[tokio::test]
    async fn propagates_storage_errors() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_network_access()
            .returning(|| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<bool, PreferencesError> =
            GetNetworkAccess::new(Arc::new(repository)).execute().await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
