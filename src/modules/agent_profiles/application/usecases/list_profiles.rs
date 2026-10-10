use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

/// Devuelve el almacén completo de perfiles (perfil activo incluido).
pub struct ListProfiles {
    store: Arc<dyn ProfileStorePort>,
}

impl ListProfiles {
    pub fn new(store: Arc<dyn ProfileStorePort>) -> Self {
        Self { store }
    }

    pub async fn execute(&self) -> Result<ProfileStore, AgentProfilesError> {
        self.store.load().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::mocks::MockProfileStoreAdapter;
    use crate::modules::agent_profiles::mocks::fixtures::{profile, store, store_port_loading};

    #[tokio::test]
    async fn returns_store_as_loaded() {
        let initial: ProfileStore = store(Some("a"), vec![profile("a", "m/1")]);
        let port: MockProfileStoreAdapter = store_port_loading(initial.clone());
        let result: Result<ProfileStore, AgentProfilesError> =
            ListProfiles::new(Arc::new(port)).execute().await;
        assert_eq!(result, Ok(initial));
    }

    #[tokio::test]
    async fn propagates_load_error() {
        let mut port: MockProfileStoreAdapter = MockProfileStoreAdapter::new();
        port.expect_load()
            .returning(|| Err(AgentProfilesError::Io("x".to_string())));
        let result: Result<ProfileStore, AgentProfilesError> =
            ListProfiles::new(Arc::new(port)).execute().await;
        assert_eq!(result, Err(AgentProfilesError::Io("x".to_string())));
    }
}
