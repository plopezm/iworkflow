use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Stores whether network access demands the access token (applied on the next start).
pub struct SaveRequireAccessToken {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl SaveRequireAccessToken {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self, required: bool) -> Result<bool, PreferencesError> {
        self.repository.save_require_access_token(required).await?;
        Ok(required)
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
            .expect_save_require_access_token()
            .withf(|required: &bool| !*required)
            .times(1)
            .returning(|_| Ok(()));

        let saved: bool = SaveRequireAccessToken::new(Arc::new(repository))
            .execute(false)
            .await
            .expect("save");

        assert!(!saved);
    }

    #[tokio::test]
    async fn propagates_storage_errors() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_require_access_token()
            .returning(|_| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<bool, PreferencesError> =
            SaveRequireAccessToken::new(Arc::new(repository))
                .execute(true)
                .await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
