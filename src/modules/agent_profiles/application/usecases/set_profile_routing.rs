use std::collections::BTreeMap;
use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

/// Reemplaza el routing (modelo y razonamiento por agente) de un perfil.
pub struct SetProfileRouting {
    store: Arc<dyn ProfileStorePort>,
}

impl SetProfileRouting {
    pub fn new(store: Arc<dyn ProfileStorePort>) -> Self {
        Self { store }
    }

    pub async fn execute(
        &self,
        name: ProfileName,
        routing: BTreeMap<String, RoutingEntry>,
    ) -> Result<Profile, AgentProfilesError> {
        Profile::validate_routing(&routing)?;
        let mut store: ProfileStore = self.store.load().await?;
        let updated: Profile = store
            .profiles
            .iter_mut()
            .find(|profile: &&mut Profile| profile.name == name)
            .map(|profile: &mut Profile| {
                profile.routing = routing;
                profile.clone()
            })
            .ok_or_else(|| AgentProfilesError::ProfileNotFound(name.to_string()))?;
        self.store.save(&store).await?;
        Ok(updated)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::mocks::MockProfileStoreAdapter;
    use crate::modules::agent_profiles::mocks::fixtures::{
        name, profile, routing_of, store, store_port_loading,
    };

    #[tokio::test]
    async fn replaces_routing_and_keeps_active() {
        let initial: ProfileStore = store(Some("a"), vec![profile("a", "m/1")]);
        let mut port: MockProfileStoreAdapter = store_port_loading(initial);
        port.expect_save()
            .withf(|saved: &ProfileStore| {
                saved.active == Some(name("a"))
                    && saved
                        .find(&name("a"))
                        .is_some_and(|p: &Profile| p.routing == routing_of("m/2"))
            })
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<Profile, AgentProfilesError> = SetProfileRouting::new(Arc::new(port))
            .execute(name("a"), routing_of("m/2"))
            .await;
        assert_eq!(result, Ok(profile("a", "m/2")));
    }

    #[tokio::test]
    async fn rejects_invalid_routing() {
        let mut port: MockProfileStoreAdapter =
            store_port_loading(store(None, vec![profile("a", "m/1")]));
        port.expect_save().times(0);
        let bad: BTreeMap<String, RoutingEntry> = BTreeMap::from([(
            "x".to_string(),
            RoutingEntry {
                model: None,
                thinking: Some("ultra".to_string()),
            },
        )]);
        let result: Result<Profile, AgentProfilesError> = SetProfileRouting::new(Arc::new(port))
            .execute(name("a"), bad)
            .await;
        assert!(matches!(result, Err(AgentProfilesError::InvalidRouting(_))));
    }

    #[tokio::test]
    async fn fails_when_profile_missing() {
        let mut port: MockProfileStoreAdapter = store_port_loading(store(None, vec![]));
        port.expect_save().times(0);
        let result: Result<Profile, AgentProfilesError> = SetProfileRouting::new(Arc::new(port))
            .execute(name("a"), routing_of("m/2"))
            .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileNotFound("a".to_string()))
        );
    }
}
