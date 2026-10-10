use serde_json::{Map, Value};

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use crate::modules::agent_profiles::domain::ports::outbound::profile_exchange_port::ProfileExchangePort;
use crate::modules::agent_profiles::infrastructure::adapters::outbound::routing_json::{
    parse_routing, routing_to_value,
};

const EXCHANGE_KIND: &str = "gentle-pi.agent_model_profile";
const EXCHANGE_VERSION: u64 = 1;

/// Sobre de intercambio `gentle-pi.agent_model_profile` v1.
#[derive(Debug, Default)]
pub struct ProfileExchangeJsonAdapter;

impl ProfileExchangeJsonAdapter {
    pub fn new() -> Self {
        Self
    }
}

fn invalid(detail: impl std::fmt::Display) -> AgentProfilesError {
    AgentProfilesError::InvalidImport(detail.to_string())
}

fn serialize(profile: &Profile) -> Result<String, AgentProfilesError> {
    Profile::validate_routing(&profile.routing)?;
    let mut root: Map<String, Value> = Map::new();
    root.insert("kind".to_string(), Value::String(EXCHANGE_KIND.to_string()));
    root.insert("version".to_string(), Value::from(EXCHANGE_VERSION));
    root.insert(
        "name".to_string(),
        Value::String(profile.name.as_str().to_string()),
    );
    root.insert("config".to_string(), routing_to_value(&profile.routing));
    let mut out: String = serde_json::to_string_pretty(&Value::Object(root))
        .map_err(|e: serde_json::Error| AgentProfilesError::Store(e.to_string()))?;
    out.push('\n');
    Ok(out)
}

fn parse(raw: &str) -> Result<Profile, AgentProfilesError> {
    let root: Value = serde_json::from_str(raw).map_err(invalid)?;
    let Value::Object(root) = root else {
        return Err(invalid("la raíz debe ser un objeto"));
    };
    if root.get("kind").and_then(Value::as_str) != Some(EXCHANGE_KIND) {
        return Err(invalid(format!("kind debe ser `{EXCHANGE_KIND}`")));
    }
    if root.get("version").and_then(Value::as_u64) != Some(EXCHANGE_VERSION) {
        return Err(invalid(format!("version debe ser {EXCHANGE_VERSION}")));
    }
    let raw_name: &str = root
        .get("name")
        .and_then(Value::as_str)
        .ok_or_else(|| invalid("falta `name`"))?;
    let name: ProfileName = ProfileName::parse(raw_name).map_err(invalid)?;
    let Some(Value::Object(config)) = root.get("config") else {
        return Err(invalid("`config` debe ser un objeto"));
    };
    let routing: std::collections::BTreeMap<String, RoutingEntry> =
        parse_routing(config).map_err(invalid)?;
    Ok(Profile::new(name, routing))
}

#[async_trait::async_trait]
impl ProfileExchangePort for ProfileExchangeJsonAdapter {
    async fn serialize_profile(&self, profile: &Profile) -> Result<String, AgentProfilesError> {
        serialize(profile)
    }

    async fn parse_profile(&self, raw: &str) -> Result<Profile, AgentProfilesError> {
        parse(raw)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_minimal_envelope() {
        let profile: Profile = parse(
            r#"{"kind":"gentle-pi.agent_model_profile","version":1,"name":"A","config":{"x":{"model":"p/m"}}}"#,
        )
        .expect("profile");
        assert_eq!(profile.name.as_str(), "A");
        assert_eq!(profile.routing["x"].model.as_deref(), Some("p/m"));
    }

    #[test]
    fn rejects_missing_name_and_wrong_version() {
        assert!(
            parse(r#"{"kind":"gentle-pi.agent_model_profile","version":1,"config":{}}"#).is_err()
        );
        assert!(
            parse(
                r#"{"kind":"gentle-pi.agent_model_profile","version":"1","name":"A","config":{}}"#
            )
            .is_err()
        );
    }

    #[test]
    fn serialize_validates_routing() {
        let profile: Profile = Profile::new(
            ProfileName::parse("A").expect("name"),
            std::collections::BTreeMap::from([(
                "x".to_string(),
                RoutingEntry {
                    model: Some(" ".to_string()),
                    thinking: None,
                },
            )]),
        );
        assert!(serialize(&profile).is_err());
    }
}
