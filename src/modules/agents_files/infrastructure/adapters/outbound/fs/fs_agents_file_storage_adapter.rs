use std::io::ErrorKind;
use std::path::{Path, PathBuf};

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::AgentsFileId;
use crate::modules::agents_files::domain::ports::outbound::agents_file_storage_port::AgentsFileStoragePort;

/// Guarda cada fichero como `<uuid>.md` dentro de `base_dir`; nunca usa nombres del usuario.
pub struct FsAgentsFileStorageAdapter {
    base_dir: PathBuf,
}

impl FsAgentsFileStorageAdapter {
    pub fn new(base_dir: PathBuf) -> Self {
        Self { base_dir }
    }

    /// Solo se aceptan rutas que cuelguen directamente de la carpeta de almacenamiento.
    async fn ensure_inside_base(&self, path: &Path) -> Result<(), AgentsFileError> {
        let Some(parent) = path.parent() else {
            return Err(outside_error(path));
        };
        if parent == self.base_dir {
            return Ok(());
        }
        match tokio::fs::canonicalize(&self.base_dir).await {
            Ok(canonical) if parent == canonical => Ok(()),
            _ => Err(outside_error(path)),
        }
    }
}

fn outside_error(path: &Path) -> AgentsFileError {
    AgentsFileError::Storage(format!(
        "la ruta {} está fuera de la carpeta de almacenamiento",
        path.display()
    ))
}

fn io_error(context: &str, path: &Path, error: std::io::Error) -> AgentsFileError {
    AgentsFileError::Storage(format!("{context} {}: {error}", path.display()))
}

#[async_trait::async_trait]
impl AgentsFileStoragePort for FsAgentsFileStorageAdapter {
    async fn store(&self, id: &AgentsFileId, content: &[u8]) -> Result<PathBuf, AgentsFileError> {
        tokio::fs::create_dir_all(&self.base_dir)
            .await
            .map_err(|e: std::io::Error| io_error("no se pudo crear", &self.base_dir, e))?;
        let base: PathBuf = tokio::fs::canonicalize(&self.base_dir)
            .await
            .map_err(|e: std::io::Error| io_error("no se pudo resolver", &self.base_dir, e))?;
        let target: PathBuf = base.join(format!("{id}.md"));
        let temp: PathBuf = base.join(format!(".{id}.md.tmp"));
        if let Err(error) = tokio::fs::write(&temp, content).await {
            let _ = tokio::fs::remove_file(&temp).await;
            return Err(io_error("no se pudo escribir", &temp, error));
        }
        if let Err(error) = tokio::fs::rename(&temp, &target).await {
            let _ = tokio::fs::remove_file(&temp).await;
            return Err(io_error("no se pudo mover a", &target, error));
        }
        Ok(target)
    }

    async fn read(&self, path: &Path) -> Result<Vec<u8>, AgentsFileError> {
        self.ensure_inside_base(path).await?;
        tokio::fs::read(path)
            .await
            .map_err(|e: std::io::Error| io_error("no se pudo leer", path, e))
    }

    async fn remove(&self, path: &Path) -> Result<(), AgentsFileError> {
        self.ensure_inside_base(path).await?;
        match tokio::fs::remove_file(path).await {
            Ok(()) => Ok(()),
            Err(error) if error.kind() == ErrorKind::NotFound => Ok(()),
            Err(error) => Err(io_error("no se pudo borrar", path, error)),
        }
    }
}
