use std::path::Path;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;

/// Acceso a `origin` mediante git: lectura de su URL y publicación de ramas.
#[async_trait::async_trait]
pub trait GitRemotePort: Send + Sync {
    /// URL de `origin`; `NoOrigin` si el repositorio no lo tiene.
    async fn origin_url(&self, repo: &Path) -> Result<String, CodeHostError>;

    /// Publica la rama local en `origin` (`git push`); la rama debe existir en `repo`.
    async fn publish_branch(&self, repo: &Path, branch: &str) -> Result<(), CodeHostError>;
}
