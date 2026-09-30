use std::path::Path;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::agents_file_option::AgentsFileOption;

/// Acceso a los ficheros AGENTS.md guardados, para el alta de proyectos nuevos.
#[async_trait::async_trait]
pub trait AgentsFilesPort: Send + Sync {
    async fn list_options(&self) -> Result<Vec<AgentsFileOption>, WorkspaceError>;

    /// Escribe el fichero como `AGENTS.md` en `root` sin sobrescribir uno existente.
    async fn install(&self, agents_file_id: &str, root: &Path) -> Result<(), WorkspaceError>;
}
