use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;

#[async_trait::async_trait]
pub trait CodeHostSettingsRepositoryPort: Send + Sync {
    /// `None` si GitLab aún no se ha configurado.
    async fn get(&self) -> Result<Option<CodeHostSettings>, CodeHostError>;
    async fn save(&self, settings: &CodeHostSettings) -> Result<(), CodeHostError>;
}
