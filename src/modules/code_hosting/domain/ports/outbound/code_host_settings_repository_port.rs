use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;

#[async_trait::async_trait]
pub trait CodeHostSettingsRepositoryPort: Send + Sync {
    /// `None` si el proveedor aún no se ha configurado.
    async fn get(
        &self,
        provider: HostingProvider,
    ) -> Result<Option<CodeHostSettings>, CodeHostError>;
    /// Ajustes de todos los proveedores configurados.
    async fn list(&self) -> Result<Vec<CodeHostSettings>, CodeHostError>;
    async fn save(&self, settings: &CodeHostSettings) -> Result<(), CodeHostError>;
}
