use std::path::{Path, PathBuf};

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::AgentsFileId;

/// Almacén en disco del contenido de los ficheros AGENTS.md.
#[async_trait::async_trait]
pub trait AgentsFileStoragePort: Send + Sync {
    /// Guarda el contenido y devuelve la ruta absoluta donde ha quedado.
    async fn store(&self, id: &AgentsFileId, content: &[u8]) -> Result<PathBuf, AgentsFileError>;
    async fn read(&self, path: &Path) -> Result<Vec<u8>, AgentsFileError>;
    /// Borra el fichero; que ya no exista no es un error.
    async fn remove(&self, path: &Path) -> Result<(), AgentsFileError>;
}
