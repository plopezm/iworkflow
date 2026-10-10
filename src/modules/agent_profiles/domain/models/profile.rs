use std::collections::BTreeMap;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;

/// Clave reservada del routing para el agente orquestador (permitida).
pub const ORCHESTRATOR_KEY: &str = "orchestrator";

/// Perfil de modelos: routing por agente (rol -> modelo + razonamiento).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Profile {
    pub name: ProfileName,
    pub routing: BTreeMap<String, RoutingEntry>,
}

impl Profile {
    pub fn new(name: ProfileName, routing: BTreeMap<String, RoutingEntry>) -> Self {
        Self { name, routing }
    }

    /// Valida claves y entradas del routing.
    pub fn validate_routing(
        routing: &BTreeMap<String, RoutingEntry>,
    ) -> Result<(), AgentProfilesError> {
        for (key, entry) in routing {
            if key.trim().is_empty() {
                return Err(AgentProfilesError::InvalidRouting(
                    "clave de agente vacía".to_string(),
                ));
            }
            entry.validate()?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(model: &str) -> RoutingEntry {
        RoutingEntry {
            model: Some(model.to_string()),
            thinking: None,
        }
    }

    #[test]
    fn orchestrator_key_is_allowed() {
        let routing: BTreeMap<String, RoutingEntry> =
            BTreeMap::from([(ORCHESTRATOR_KEY.to_string(), entry("a/b"))]);
        assert_eq!(Profile::validate_routing(&routing), Ok(()));
    }

    #[test]
    fn rejects_empty_key() {
        let routing: BTreeMap<String, RoutingEntry> =
            BTreeMap::from([(" ".to_string(), entry("a/b"))]);
        assert!(Profile::validate_routing(&routing).is_err());
    }

    #[test]
    fn rejects_invalid_entry() {
        let routing: BTreeMap<String, RoutingEntry> = BTreeMap::from([(
            "sdd-apply".to_string(),
            RoutingEntry {
                model: None,
                thinking: Some("ultra".to_string()),
            },
        )]);
        assert!(Profile::validate_routing(&routing).is_err());
    }
}
