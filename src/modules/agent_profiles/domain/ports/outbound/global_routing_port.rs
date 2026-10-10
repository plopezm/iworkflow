use std::collections::BTreeMap;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;

/// Acceso al routing global (`models.json`).
#[async_trait::async_trait]
pub trait GlobalRoutingPort: Send + Sync {
    async fn load(&self) -> Result<BTreeMap<String, RoutingEntry>, AgentProfilesError>;
    async fn save(
        &self,
        routing: &BTreeMap<String, RoutingEntry>,
    ) -> Result<(), AgentProfilesError>;
}
