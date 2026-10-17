use std::path::Path;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;

/// Solo lectura: la app nunca publica ramas (`git push`).
#[async_trait::async_trait]
pub trait GitRemotePort: Send + Sync {
    /// URL de `origin`; `NoOrigin` si el repositorio no lo tiene.
    async fn origin_url(&self, repo: &Path) -> Result<String, CodeHostError>;
}
