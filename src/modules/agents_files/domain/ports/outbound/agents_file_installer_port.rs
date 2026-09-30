use std::path::Path;

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::install_outcome::InstallOutcome;

/// Escribe un `AGENTS.md` en la raíz de un proyecto.
#[async_trait::async_trait]
pub trait AgentsFileInstallerPort: Send + Sync {
    /// Devuelve `TargetExists` si ya hay un `AGENTS.md` y `overwrite` es `false`.
    async fn install(
        &self,
        content: &[u8],
        dir: &Path,
        overwrite: bool,
    ) -> Result<InstallOutcome, AgentsFileError>;
}
