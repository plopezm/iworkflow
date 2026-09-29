use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::profile_catalog::ProfileCatalog;

#[async_trait::async_trait]
pub trait ProfileCatalogPort: Send + Sync {
    async fn list(&self) -> Result<ProfileCatalog, AgentSettingsError>;
}
