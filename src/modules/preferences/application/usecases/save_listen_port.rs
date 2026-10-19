use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::listen_port::ListenPort;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Validates and stores the server port (applied on the next start).
pub struct SaveListenPort {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl SaveListenPort {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self, port: u16) -> Result<ListenPort, PreferencesError> {
        let port: ListenPort = ListenPort::new(port)?;
        self.repository.save_listen_port(&port).await?;
        Ok(port)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::preferences::mocks::MockPreferencesRepository;

    #[tokio::test]
    async fn saves_a_valid_port() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_listen_port()
            .withf(|port: &ListenPort| port.get() == 9100)
            .times(1)
            .returning(|_| Ok(()));

        let saved: ListenPort = SaveListenPort::new(Arc::new(repository))
            .execute(9100)
            .await
            .expect("save");

        assert_eq!(saved.get(), 9100);
    }

    #[tokio::test]
    async fn rejects_a_privileged_port_without_touching_storage() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository.expect_save_listen_port().never();

        let result: Result<ListenPort, PreferencesError> =
            SaveListenPort::new(Arc::new(repository)).execute(80).await;

        assert_eq!(result, Err(PreferencesError::InvalidPort(80)));
    }

    #[tokio::test]
    async fn propagates_storage_errors() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_listen_port()
            .returning(|_| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<ListenPort, PreferencesError> =
            SaveListenPort::new(Arc::new(repository))
                .execute(9100)
                .await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
