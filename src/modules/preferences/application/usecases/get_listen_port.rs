use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::listen_port::ListenPort;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Reads the port the user chose for the server, if any.
pub struct GetListenPort {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl GetListenPort {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<Option<ListenPort>, PreferencesError> {
        self.repository.listen_port().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::preferences::mocks::MockPreferencesRepository;

    #[tokio::test]
    async fn returns_the_stored_port() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_listen_port()
            .returning(|| Ok(Some(ListenPort::new(9100).expect("port"))));

        let port: Option<ListenPort> = GetListenPort::new(Arc::new(repository))
            .execute()
            .await
            .expect("port");

        assert_eq!(port.map(|p: ListenPort| p.get()), Some(9100));
    }

    #[tokio::test]
    async fn propagates_storage_errors() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_listen_port()
            .returning(|| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<Option<ListenPort>, PreferencesError> =
            GetListenPort::new(Arc::new(repository)).execute().await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
