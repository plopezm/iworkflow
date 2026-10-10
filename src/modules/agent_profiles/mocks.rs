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
