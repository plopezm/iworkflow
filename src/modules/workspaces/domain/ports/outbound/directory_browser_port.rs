use std::path::Path;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::directory_listing::DirectoryListing;

#[async_trait::async_trait]
pub trait DirectoryBrowserPort: Send + Sync {
    /// Lista el contenido de `path`. Si `path` no existe (por ejemplo, la ruta de un
    /// workspace aún no creado), se lista la carpeta existente más cercana entre sus
    /// ancestros. Falla con `PathNotFound` si ni la carpeta ni ningún ancestro existen,
    /// o con `Io` ante otros errores de lectura.
    async fn list(&self, path: &Path) -> Result<DirectoryListing, WorkspaceError>;
}
