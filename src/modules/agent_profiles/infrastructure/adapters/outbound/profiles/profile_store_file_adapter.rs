use std::collections::BTreeMap;
use std::path::PathBuf;

use serde_json::{Map, Value};

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;
use crate::modules::agent_profiles::infrastructure::adapters::outbound::fs_util::{
    atomic_write, read_optional,
};
use crate::modules::agent_profiles::infrastructure::adapters::outbound::routing_json::{
    parse_routing, routing_to_value,
};

const PROFILES_FILE: &str = "profiles.json";
const STORE_KIND: &str = "gentle-pi.agent_model_profiles";
const STORE_VERSION: u64 = 1;

/// Lee y escribe `<gentle_pi_config_home>/profiles.json`. Fichero ausente = almacén vacío.
pub struct ProfileStoreFileAdapter {
    path: PathBuf,
}

impl ProfileStoreFileAdapter {
    pub fn new(gentle_pi_config_home: PathBuf) -> Self {
        Self {
            path: gentle_pi_config_home.join(PROFILES_FILE),
        }
    }
}

fn store_error(detail: impl std::fmt::Display) -> AgentProfilesError {
    AgentProfilesError::Store(format!("profiles.json inválido: {detail}"))
}

fn parse_store(raw: &str) -> Result<ProfileStore, AgentProfilesError> {
    let root: Value = serde_json::from_str(raw).map_err(store_error)?;
    let Value::Object(root) = root else {
        return Err(store_error("la raíz debe ser un objeto"));
    };
    if let Some(kind) = root.get("kind").filter(|kind: &&Value| *kind != STORE_KIND) {
        return Err(store_error(format!("kind inesperado: {kind}")));
    }
    let active: Option<ProfileName> = match root.get("active") {
        None | Some(Value::Null) => None,
        Some(Value::String(name)) => Some(ProfileName::parse(name)?),
        Some(_) => return Err(store_error("`active` debe ser texto")),
    };
    let profiles_map: &Map<String, Value> = match root.get("profiles") {
        None | Some(Value::Null) => {
            return Ok(ProfileStore {
                active,
                profiles: Vec::new(),
            });
        }
        Some(Value::Object(map)) => map,
        Some(_) => return Err(store_error("`profiles` debe ser un objeto")),
    };
    let mut profiles: Vec<Profile> = Vec::with_capacity(profiles_map.len());
    for (raw_name, config) in profiles_map {
        let name: ProfileName = ProfileName::parse(raw_name)
            .map_err(|e: AgentProfilesError| store_error(format!("perfil `{raw_name}`: {e}")))?;
        let Value::Object(config) = config else {
            return Err(store_error(format!(
                "perfil `{raw_name}`: se esperaba un objeto"
            )));
        };
        let routing: BTreeMap<String, RoutingEntry> = parse_routing(config)
            .map_err(|e: String| store_error(format!("perfil `{raw_name}`: {e}")))?;
        profiles.push(Profile::new(name, routing));
    }
    profiles.sort_by(|a: &Profile, b: &Profile| a.name.cmp(&b.name));
    Ok(ProfileStore { active, profiles })
}

fn serialize_store(store: &ProfileStore) -> Result<String, AgentProfilesError> {
    let mut sorted: Vec<&Profile> = store.profiles.iter().collect();
    sorted.sort_by(|a: &&Profile, b: &&Profile| a.name.cmp(&b.name));
    let mut profiles: Map<String, Value> = Map::new();
    for profile in sorted {
        profiles.insert(
            profile.name.as_str().to_string(),
            routing_to_value(&profile.routing),
        );
    }
    let mut root: Map<String, Value> = Map::new();
    root.insert("kind".to_string(), Value::String(STORE_KIND.to_string()));
    root.insert("version".to_string(), Value::from(STORE_VERSION));
    if let Some(active) = &store.active {
        root.insert(
            "active".to_string(),
            Value::String(active.as_str().to_string()),
        );
    }
    root.insert("profiles".to_string(), Value::Object(profiles));
    let mut out: String = serde_json::to_string_pretty(&Value::Object(root))
        .map_err(|e: serde_json::Error| AgentProfilesError::Store(e.to_string()))?;
    out.push('\n');
    Ok(out)
}

#[async_trait::async_trait]
impl ProfileStorePort for ProfileStoreFileAdapter {
    async fn load(&self) -> Result<ProfileStore, AgentProfilesError> {
        match read_optional(&self.path).await? {
            Some(raw) => parse_store(&raw),
            None => Ok(ProfileStore::default()),
        }
    }

    async fn save(&self, store: &ProfileStore) -> Result<(), AgentProfilesError> {
        let content: String = serialize_store(store)?;
        atomic_write(&self.path, &content).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_empty_object_as_empty_store() {
        assert_eq!(parse_store("{}").expect("store"), ProfileStore::default());
    }

    #[test]
    fn rejects_wrong_kind_and_non_object_root() {
        assert!(parse_store(r#"{"kind":"other"}"#).is_err());
        assert!(parse_store("[]").is_err());
        assert!(parse_store(r#"{"active":3}"#).is_err());
        assert!(parse_store(r#"{"profiles":[]}"#).is_err());
    }

    #[test]
    fn serialization_is_deterministic_and_round_trips() {
        let raw: &str = r#"{"profiles":{"B":{"z":{"model":"m"},"a":"x/y"},"A":{}},"active":"B"}"#;
        let store: ProfileStore = parse_store(raw).expect("store");
        let first: String = serialize_store(&store).expect("first");
        let second: String = serialize_store(&parse_store(&first).expect("again")).expect("second");
        assert_eq!(first, second);
        assert!(first.find("\"A\": {").expect("A") < first.find("\"B\": {").expect("B"));
        assert!(first.find("\"a\": {").expect("a") < first.find("\"z\": {").expect("z"));
    }
}
