use std::path::Path;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;

/// Acceso a `origin` mediante git: lectura de su URL y publicación de ramas.
#[async_trait::async_trait]
pub trait GitRemotePort: Send + Sync {
    /// URL de `origin`; `NoOrigin` si el repositorio no lo tiene.
    async fn origin_url(&self, repo: &Path) -> Result<String, CodeHostError>;

    /// Publica la rama local con `git push` hacia el proyecto del proveedor, autenticándose con
    /// el token de `settings` (no con las credenciales de git del sistema). La rama debe existir
    /// en `repo`; `project` es `namespace/proyecto` u `propietario/repositorio`.
    async fn publish_branch(
        &self,
        repo: &Path,
        branch: &str,
        settings: &CodeHostSettings,
        project: &str,
    ) -> Result<(), CodeHostError>;
}
