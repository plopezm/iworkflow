use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Reads whether network access demands the access token (it does unless switched off).
pub struct GetRequireAccessToken {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl GetRequireAccessToken {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<bool, PreferencesError> {
        self.repository.require_access_token().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::preferences::mocks::MockPreferencesRepository;

    #[tokio::test]
    async fn returns_the_stored_choice() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_require_access_token()
            .returning(|| Ok(false));

        let required: bool = GetRequireAccessToken::new(Arc::new(repository))
            .execute()
            .await
            .expect("get");

        assert!(!required);
    }

    #[tokio::test]
    async fn propagates_storage_errors() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_require_access_token()
            .returning(|| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<bool, PreferencesError> =
            GetRequireAccessToken::new(Arc::new(repository))
                .execute()
                .await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
