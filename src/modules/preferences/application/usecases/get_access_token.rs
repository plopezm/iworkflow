use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::access_token::AccessToken;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Returns the access token, generating and storing one the first time it is needed.
pub struct GetAccessToken {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl GetAccessToken {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<AccessToken, PreferencesError> {
        if let Some(token) = self.repository.access_token().await? {
            return Ok(token);
        }
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
    async fn returns_the_stored_token_without_writing() {
        let stored: AccessToken = AccessToken::generate();
        let expected: AccessToken = stored.clone();
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_access_token()
            .returning(move || Ok(Some(stored.clone())));
        repository.expect_save_access_token().times(0);

        let token: AccessToken = GetAccessToken::new(Arc::new(repository))
            .execute()
            .await
            .expect("get");

        assert_eq!(token, expected);
    }

    #[tokio::test]
    async fn generates_and_stores_a_token_the_first_time() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository.expect_access_token().returning(|| Ok(None));
        repository
            .expect_save_access_token()
            .times(1)
            .returning(|_| Ok(()));

        let token: AccessToken = GetAccessToken::new(Arc::new(repository))
            .execute()
            .await
            .expect("get");

        assert!(AccessToken::parse(token.as_str()).is_ok());
    }

    #[tokio::test]
    async fn does_not_hand_out_a_token_that_failed_to_persist() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository.expect_access_token().returning(|| Ok(None));
        repository
            .expect_save_access_token()
            .returning(|_| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<AccessToken, PreferencesError> =
            GetAccessToken::new(Arc::new(repository)).execute().await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
