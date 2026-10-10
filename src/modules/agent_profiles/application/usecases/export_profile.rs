use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::ports::outbound::profile_exchange_port::ProfileExchangePort;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

/// Serializa un perfil existente al formato de intercambio.
pub struct ExportProfile {
    store: Arc<dyn ProfileStorePort>,
    exchange: Arc<dyn ProfileExchangePort>,
}

impl ExportProfile {
    pub fn new(store: Arc<dyn ProfileStorePort>, exchange: Arc<dyn ProfileExchangePort>) -> Self {
        Self { store, exchange }
    }

    pub async fn execute(&self, name: ProfileName) -> Result<String, AgentProfilesError> {
        let store: ProfileStore = self.store.load().await?;
        let profile: &Profile = store
            .find(&name)
            .ok_or_else(|| AgentProfilesError::ProfileNotFound(name.to_string()))?;
        self.exchange.serialize_profile(profile).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::mocks::MockProfileExchangeAdapter;
    use crate::modules::agent_profiles::mocks::fixtures::{
        name, profile, store, store_port_loading,
    };

    #[tokio::test]
    async fn serializes_existing_profile() {
        let port = store_port_loading(store(None, vec![profile("a", "m/1")]));
        let mut exchange: MockProfileExchangeAdapter = MockProfileExchangeAdapter::new();
        exchange
            .expect_serialize_profile()
            .withf(|p| p == &profile("a", "m/1"))
            .times(1)
            .returning(|_| Ok("{json}".to_string()));
        let result: Result<String, AgentProfilesError> =
            ExportProfile::new(Arc::new(port), Arc::new(exchange))
                .execute(name("a"))
                .await;
        assert_eq!(result, Ok("{json}".to_string()));
    }

    #[tokio::test]
    async fn fails_when_missing_without_serializing() {
        let port = store_port_loading(store(None, vec![]));
        let mut exchange: MockProfileExchangeAdapter = MockProfileExchangeAdapter::new();
        exchange.expect_serialize_profile().times(0);
        let result: Result<String, AgentProfilesError> =
            ExportProfile::new(Arc::new(port), Arc::new(exchange))
                .execute(name("a"))
                .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileNotFound("a".to_string()))
        );
    }
}
