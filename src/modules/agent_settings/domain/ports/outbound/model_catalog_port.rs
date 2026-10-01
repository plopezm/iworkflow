use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;

#[async_trait::async_trait]
pub trait ModelCatalogPort: Send + Sync {
    /// Modelos disponibles en el ejecutable del `runtime` indicado.
    async fn list(&self, runtime: AgentRuntime) -> Result<Vec<ModelRef>, AgentSettingsError>;
}
