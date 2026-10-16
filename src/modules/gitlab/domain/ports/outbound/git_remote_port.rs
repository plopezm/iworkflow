use std::path::Path;

use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;

/// Solo lectura: la app nunca publica ramas (`git push`).
#[async_trait::async_trait]
pub trait GitRemotePort: Send + Sync {
    /// URL de `origin`; `NoOrigin` si el repositorio no lo tiene.
    async fn origin_url(&self, repo: &Path) -> Result<String, GitLabError>;
}
