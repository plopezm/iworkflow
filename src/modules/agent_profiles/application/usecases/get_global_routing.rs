use std::collections::BTreeMap;
use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use crate::modules::agent_profiles::domain::ports::outbound::global_routing_port::GlobalRoutingPort;

/// Lee el routing global (`models.json`).
pub struct GetGlobalRouting {
    routing: Arc<dyn GlobalRoutingPort>,
}

impl GetGlobalRouting {
    pub fn new(routing: Arc<dyn GlobalRoutingPort>) -> Self {
        Self { routing }
    }

    pub async fn execute(&self) -> Result<BTreeMap<String, RoutingEntry>, AgentProfilesError> {
        self.routing.load().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::mocks::MockGlobalRoutingAdapter;
    use crate::modules::agent_profiles::mocks::fixtures::routing_of;

    #[tokio::test]
    async fn returns_loaded_routing() {
        let mut port: MockGlobalRoutingAdapter = MockGlobalRoutingAdapter::new();
        port.expect_load().returning(|| Ok(routing_of("m/1")));
        let result: Result<BTreeMap<String, RoutingEntry>, AgentProfilesError> =
            GetGlobalRouting::new(Arc::new(port)).execute().await;
        assert_eq!(result, Ok(routing_of("m/1")));
    }

    #[tokio::test]
    async fn propagates_error() {
        let mut port: MockGlobalRoutingAdapter = MockGlobalRoutingAdapter::new();
        port.expect_load()
            .returning(|| Err(AgentProfilesError::Io("x".to_string())));
        let result: Result<BTreeMap<String, RoutingEntry>, AgentProfilesError> =
            GetGlobalRouting::new(Arc::new(port)).execute().await;
        assert_eq!(result, Err(AgentProfilesError::Io("x".to_string())));
    }
}
