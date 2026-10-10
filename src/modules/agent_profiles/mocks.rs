use std::collections::BTreeMap;

use mockall::mock;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::agent_role::{AgentRole, NewAgentRole};
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use crate::modules::agent_profiles::domain::ports::outbound::agent_role_port::AgentRolePort;
use crate::modules::agent_profiles::domain::ports::outbound::global_routing_port::GlobalRoutingPort;
use crate::modules::agent_profiles::domain::ports::outbound::model_options_port::{
    ModelOption, ModelOptionsPort,
};
use crate::modules::agent_profiles::domain::ports::outbound::profile_exchange_port::ProfileExchangePort;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

mock! {
    pub ProfileStoreAdapter {}

    #[async_trait::async_trait]
    impl ProfileStorePort for ProfileStoreAdapter {
        async fn load(&self) -> Result<ProfileStore, AgentProfilesError>;
        async fn save(&self, store: &ProfileStore) -> Result<(), AgentProfilesError>;
    }
}

mock! {
    pub GlobalRoutingAdapter {}

    #[async_trait::async_trait]
    impl GlobalRoutingPort for GlobalRoutingAdapter {
        async fn load(&self) -> Result<BTreeMap<String, RoutingEntry>, AgentProfilesError>;
        async fn save(
            &self,
            routing: &BTreeMap<String, RoutingEntry>,
        ) -> Result<(), AgentProfilesError>;
    }
}

mock! {
    pub AgentRoleAdapter {}

    #[async_trait::async_trait]
    impl AgentRolePort for AgentRoleAdapter {
        async fn list(&self) -> Result<Vec<AgentRole>, AgentProfilesError>;
        async fn create(&self, role: &NewAgentRole) -> Result<(), AgentProfilesError>;
    }
}

mock! {
    pub ModelOptionsAdapter {}

    #[async_trait::async_trait]
    impl ModelOptionsPort for ModelOptionsAdapter {
        async fn list(&self) -> Result<Vec<ModelOption>, AgentProfilesError>;
    }
}

mock! {
    pub ProfileExchangeAdapter {}

    #[async_trait::async_trait]
    impl ProfileExchangePort for ProfileExchangeAdapter {
        async fn serialize_profile(&self, profile: &Profile) -> Result<String, AgentProfilesError>;
        async fn parse_profile(&self, raw: &str) -> Result<Profile, AgentProfilesError>;
    }
}

/// Helpers de construcción para los tests de casos de uso.
pub mod fixtures {
    use std::collections::BTreeMap;

    use super::*;
    use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;

    pub fn name(raw: &str) -> ProfileName {
        ProfileName::parse(raw).expect("nombre de test válido")
    }

    pub fn routing_of(model: &str) -> BTreeMap<String, RoutingEntry> {
        BTreeMap::from([(
            "sdd-apply".to_string(),
            RoutingEntry {
                model: Some(model.to_string()),
                thinking: Some("high".to_string()),
            },
        )])
    }

    pub fn profile(raw: &str, model: &str) -> Profile {
        Profile::new(name(raw), routing_of(model))
    }

    pub fn store(active: Option<&str>, profiles: Vec<Profile>) -> ProfileStore {
        ProfileStore {
            active: active.map(name),
            profiles,
        }
    }

    /// Mock de almacén que devuelve `initial` en `load` y no espera `save`.
    pub fn store_port_loading(initial: ProfileStore) -> MockProfileStoreAdapter {
        let mut port: MockProfileStoreAdapter = MockProfileStoreAdapter::new();
        port.expect_load().returning(move || Ok(initial.clone()));
        port
    }
}
