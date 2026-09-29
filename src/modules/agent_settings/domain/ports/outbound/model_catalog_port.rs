use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;

#[async_trait::async_trait]
pub trait ModelCatalogPort: Send + Sync {
    async fn list(&self) -> Result<Vec<ModelRef>, AgentSettingsError>;
}
