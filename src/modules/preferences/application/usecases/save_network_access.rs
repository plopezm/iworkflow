use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Stores whether the server listens on every network interface (applied on the next start).
pub struct SaveNetworkAccess {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl SaveNetworkAccess {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self, enabled: bool) -> Result<bool, PreferencesError> {
        self.repository.save_network_access(enabled).await?;
        Ok(enabled)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::preferences::mocks::MockPreferencesRepository;

    #[tokio::test]
    async fn saves_the_choice() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_network_access()
            .withf(|enabled: &bool| *enabled)
            .times(1)
            .returning(|_| Ok(()));

        let saved: bool = SaveNetworkAccess::new(Arc::new(repository))
            .execute(true)
            .await
            .expect("save");

        assert!(saved);
    }

    #[tokio::test]
    async fn propagates_storage_errors() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_network_access()
            .returning(|_| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<bool, PreferencesError> = SaveNetworkAccess::new(Arc::new(repository))
            .execute(false)
            .await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
