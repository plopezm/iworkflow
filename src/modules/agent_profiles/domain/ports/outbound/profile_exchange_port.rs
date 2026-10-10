use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;

/// Serialización del sobre `gentle-pi.agent_model_profile` v1
/// (`{kind, version, name, config}`), fuera del dominio.
#[async_trait::async_trait]
pub trait ProfileExchangePort: Send + Sync {
    async fn serialize_profile(&self, profile: &Profile) -> Result<String, AgentProfilesError>;
    async fn parse_profile(&self, raw: &str) -> Result<Profile, AgentProfilesError>;
}
