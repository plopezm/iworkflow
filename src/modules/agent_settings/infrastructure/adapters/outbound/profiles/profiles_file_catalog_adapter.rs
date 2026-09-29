use std::collections::HashMap;
use std::path::PathBuf;

use serde::Deserialize;
use serde_json::Value;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::profile_catalog::ProfileCatalog;
use crate::modules::agent_settings::domain::ports::outbound::profile_catalog_port::ProfileCatalogPort;

const PROFILES_FILE: &str = "profiles.json";

#[derive(Debug, Deserialize)]
struct ProfilesFile {
    active: Option<String>,
    #[serde(default)]
    profiles: HashMap<String, Value>,
}

/// Lee `<gentle_pi_config_home>/profiles.json`. Fichero ausente = catálogo vacío.
pub struct ProfilesFileCatalogAdapter {
    profiles_path: PathBuf,
}

impl ProfilesFileCatalogAdapter {
    pub fn new(gentle_pi_config_home: PathBuf) -> Self {
        Self {
            profiles_path: gentle_pi_config_home.join(PROFILES_FILE),
        }
    }
}

#[async_trait::async_trait]
impl ProfileCatalogPort for ProfilesFileCatalogAdapter {
    async fn list(&self) -> Result<ProfileCatalog, AgentSettingsError> {
        let content: String = match tokio::fs::read_to_string(&self.profiles_path).await {
            Ok(content) => content,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                return Ok(ProfileCatalog::default());
            }
            Err(e) => return Err(AgentSettingsError::Io(e.to_string())),
        };
        parse_profiles_file(&content)
    }
}

fn parse_profiles_file(content: &str) -> Result<ProfileCatalog, AgentSettingsError> {
    let parsed: ProfilesFile = serde_json::from_str(content)
        .map_err(|e| AgentSettingsError::Catalog(format!("profiles.json inválido: {e}")))?;
    let mut names: Vec<String> = parsed.profiles.into_keys().collect();
    names.sort();
    Ok(ProfileCatalog {
        active: parsed.active,
        names,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_active_and_sorted_names() {
        let content: &str = r#"{
            "kind": "gentle-pi.agent_model_profiles",
            "version": 1,
            "active": "Claude",
            "profiles": {"GPT": {}, "Claude": {}}
        }"#;

        let catalog: ProfileCatalog = parse_profiles_file(content).expect("catalog");

        assert_eq!(catalog.active.as_deref(), Some("Claude"));
        assert_eq!(catalog.names, vec!["Claude".to_string(), "GPT".to_string()]);
    }

    #[test]
    fn missing_active_is_none() {
        let content: &str =
            r#"{"kind": "gentle-pi.agent_model_profiles", "version": 1, "profiles": {}}"#;

        let catalog: ProfileCatalog = parse_profiles_file(content).expect("catalog");

        assert_eq!(catalog.active, None);
        assert!(catalog.names.is_empty());
    }

    #[test]
    fn invalid_json_is_catalog_error() {
        let result: Result<ProfileCatalog, AgentSettingsError> = parse_profiles_file("not json");

        assert!(matches!(result, Err(AgentSettingsError::Catalog(_))));
    }
}
