use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::access_token::AccessToken;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Replaces the access token; every device using the old one must authenticate again.
pub struct RegenerateAccessToken {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl RegenerateAccessToken {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<AccessToken, PreferencesError> {
        let token: AccessToken = AccessToken::generate();
        self.repository.save_access_token(&token).await?;
        Ok(token)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::preferences::mocks::MockPreferencesRepository;

    #[tokio::test]
    async fn stores_and_returns_a_new_token() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_access_token()
            .times(1)
            .returning(|_| Ok(()));

        let token: AccessToken = RegenerateAccessToken::new(Arc::new(repository))
            .execute()
            .await
            .expect("regenerate");

        assert!(AccessToken::parse(token.as_str()).is_ok());
    }

    #[tokio::test]
    async fn propagates_storage_errors() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_access_token()
            .returning(|_| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<AccessToken, PreferencesError> =
            RegenerateAccessToken::new(Arc::new(repository))
                .execute()
                .await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
