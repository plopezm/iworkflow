use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;

#[async_trait::async_trait]
pub trait AgentSettingsRepositoryPort: Send + Sync {
    /// Fila ausente = `AgentSettings::default()`.
    async fn get(&self) -> Result<AgentSettings, AgentSettingsError>;
    /// Upsert de la única fila de ajustes.
    async fn save(&self, settings: &AgentSettings) -> Result<(), AgentSettingsError>;
}
