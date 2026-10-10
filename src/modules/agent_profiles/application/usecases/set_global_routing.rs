use std::collections::BTreeMap;
use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use crate::modules::agent_profiles::domain::ports::outbound::global_routing_port::GlobalRoutingPort;

/// Valida y guarda el routing global (`models.json`).
pub struct SetGlobalRouting {
    routing: Arc<dyn GlobalRoutingPort>,
}

impl SetGlobalRouting {
    pub fn new(routing: Arc<dyn GlobalRoutingPort>) -> Self {
        Self { routing }
    }

    pub async fn execute(
        &self,
        routing: BTreeMap<String, RoutingEntry>,
    ) -> Result<(), AgentProfilesError> {
        Profile::validate_routing(&routing)?;
        self.routing.save(&routing).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::mocks::MockGlobalRoutingAdapter;
    use crate::modules::agent_profiles::mocks::fixtures::routing_of;

    #[tokio::test]
    async fn saves_valid_routing() {
        let mut port: MockGlobalRoutingAdapter = MockGlobalRoutingAdapter::new();
        port.expect_save()
            .withf(|r: &BTreeMap<String, RoutingEntry>| r == &routing_of("m/1"))
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<(), AgentProfilesError> = SetGlobalRouting::new(Arc::new(port))
            .execute(routing_of("m/1"))
            .await;
        assert_eq!(result, Ok(()));
    }

    #[tokio::test]
    async fn rejects_invalid_routing_without_saving() {
        let mut port: MockGlobalRoutingAdapter = MockGlobalRoutingAdapter::new();
        port.expect_save().times(0);
        let bad: BTreeMap<String, RoutingEntry> =
            BTreeMap::from([(" ".to_string(), RoutingEntry::default())]);
        let result: Result<(), AgentProfilesError> =
            SetGlobalRouting::new(Arc::new(port)).execute(bad).await;
        assert!(matches!(result, Err(AgentProfilesError::InvalidRouting(_))));
    }
}
