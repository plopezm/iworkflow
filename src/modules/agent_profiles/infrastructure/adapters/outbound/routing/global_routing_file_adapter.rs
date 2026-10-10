use std::collections::BTreeMap;
use std::path::PathBuf;

use serde_json::Value;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use crate::modules::agent_profiles::domain::ports::outbound::global_routing_port::GlobalRoutingPort;
use crate::modules::agent_profiles::infrastructure::adapters::outbound::fs_util::{
    atomic_write, read_optional,
};
use crate::modules::agent_profiles::infrastructure::adapters::outbound::routing_json::{
    parse_routing, routing_to_value,
};

const MODELS_FILE: &str = "models.json";

/// Lee y escribe `<gentle_pi_config_home>/models.json` (mapa plano agente -> entrada).
pub struct GlobalRoutingFileAdapter {
    path: PathBuf,
}

impl GlobalRoutingFileAdapter {
    pub fn new(gentle_pi_config_home: PathBuf) -> Self {
        Self {
            path: gentle_pi_config_home.join(MODELS_FILE),
        }
    }
}

fn store_error(detail: impl std::fmt::Display) -> AgentProfilesError {
    AgentProfilesError::Store(format!("models.json inválido: {detail}"))
}

fn parse_models(raw: &str) -> Result<BTreeMap<String, RoutingEntry>, AgentProfilesError> {
    let root: Value = serde_json::from_str(raw).map_err(store_error)?;
    let Value::Object(map) = root else {
        return Err(store_error("la raíz debe ser un objeto"));
    };
    parse_routing(&map).map_err(store_error)
}

fn serialize_models(
    routing: &BTreeMap<String, RoutingEntry>,
) -> Result<String, AgentProfilesError> {
    Profile::validate_routing(routing)?;
    let mut out: String = serde_json::to_string_pretty(&routing_to_value(routing))
        .map_err(|e: serde_json::Error| AgentProfilesError::Store(e.to_string()))?;
    out.push('\n');
    Ok(out)
}

#[async_trait::async_trait]
impl GlobalRoutingPort for GlobalRoutingFileAdapter {
    async fn load(&self) -> Result<BTreeMap<String, RoutingEntry>, AgentProfilesError> {
        match read_optional(&self.path).await? {
            Some(raw) => parse_models(&raw),
            None => Ok(BTreeMap::new()),
        }
    }

    async fn save(
        &self,
        routing: &BTreeMap<String, RoutingEntry>,
    ) -> Result<(), AgentProfilesError> {
        let content: String = serialize_models(routing)?;
        atomic_write(&self.path, &content).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_mixed_legacy_and_object_entries() {
        let routing: BTreeMap<String, RoutingEntry> =
            parse_models(r#"{"a":"p/m","b":{"thinking":"low"}}"#).expect("routing");
        assert_eq!(routing["a"].model.as_deref(), Some("p/m"));
        assert_eq!(routing["b"].thinking.as_deref(), Some("low"));
    }

    #[test]
    fn rejects_non_object_root() {
        assert!(parse_models("[]").is_err());
    }

    #[test]
    fn serialize_ends_with_newline_and_rejects_invalid_entry() {
        let mut routing: BTreeMap<String, RoutingEntry> = BTreeMap::new();
        routing.insert("a".to_string(), RoutingEntry::default());
        assert!(serialize_models(&routing).expect("ok").ends_with("}\n"));
        routing.insert(
            "b".to_string(),
            RoutingEntry {
                model: None,
                thinking: Some("ultra".to_string()),
            },
        );
        assert!(serialize_models(&routing).is_err());
    }
}
