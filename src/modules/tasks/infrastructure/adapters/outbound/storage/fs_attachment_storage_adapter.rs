use std::path::{Component, Path, PathBuf};

use base64::Engine as _;
use base64::engine::general_purpose::STANDARD;
use uuid::Uuid;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;

/// Guarda las imágenes en `<base_dir>/<task_id>/<attachment_id>.<ext>`.
pub struct FsAttachmentStorageAdapter {
    base_dir: PathBuf,
}

impl FsAttachmentStorageAdapter {
    pub fn new(base_dir: PathBuf) -> Self {
        Self { base_dir }
    }

    /// Resuelve una ruta relativa dentro del directorio base rechazando cualquier intento de salir de él.
    fn resolve(&self, relative: &str) -> Result<PathBuf, TaskError> {
        let path: &Path = Path::new(relative);
        let mut has_normal: bool = false;
        for component in path.components() {
            match component {
                Component::Normal(_) => has_normal = true,
                _ => {
                    return Err(TaskError::Storage(format!(
                        "ruta de adjunto no válida: {relative}"
                    )));
                }
            }
        }
        if !has_normal {
            return Err(TaskError::Storage(format!(
                "ruta de adjunto no válida: {relative}"
            )));
        }
        Ok(self.base_dir.join(path))
    }
}

fn extension_for(mime_type: &str) -> Result<&'static str, TaskError> {
    match mime_type {
        "image/png" => Ok("png"),
        "image/jpeg" => Ok("jpg"),
        "image/gif" => Ok("gif"),
        "image/webp" => Ok("webp"),
        other => Err(TaskError::Storage(format!(
            "tipo de imagen no admitido: {other}"
        ))),
    }
}

fn io_error(context: &str, error: std::io::Error) -> TaskError {
    TaskError::Storage(format!("{context}: {error}"))
}

#[async_trait::async_trait]
impl AttachmentStoragePort for FsAttachmentStorageAdapter {
    async fn save(
        &self,
        task_id: TaskId,
        attachment_id: &str,
        image: &ImageAttachment,
    ) -> Result<String, TaskError> {
        let extension: &str = extension_for(&image.mime_type)?;
        let relative: String = format!("{task_id}/{attachment_id}.{extension}");
        let target: PathBuf = self.resolve(&relative)?;
        // El id no puede introducir subdirectorios: la ruta debe tener exactamente dos niveles.
        if Path::new(&relative).components().count() != 2 {
            return Err(TaskError::Storage(format!(
                "identificador de adjunto no válido: {attachment_id}"
            )));
        }
        let bytes: Vec<u8> = STANDARD
            .decode(image.data.as_bytes())
            .map_err(|e| TaskError::Storage(format!("{}: base64 inválido: {e}", image.name)))?;
        let dir: PathBuf = self.base_dir.join(task_id.to_string());
        tokio::fs::create_dir_all(&dir)
            .await
            .map_err(|e| io_error("no se pudo crear el directorio de adjuntos", e))?;
        let temp: PathBuf = dir.join(format!(".{}.tmp", Uuid::new_v4()));
        if let Err(error) = tokio::fs::write(&temp, &bytes).await {
            let _ = tokio::fs::remove_file(&temp).await;
            return Err(io_error("no se pudo escribir el adjunto", error));
        }
        if let Err(error) = tokio::fs::rename(&temp, &target).await {
            let _ = tokio::fs::remove_file(&temp).await;
            return Err(io_error("no se pudo guardar el adjunto", error));
        }
        Ok(relative)
    }

    async fn read(&self, path: &str) -> Result<String, TaskError> {
        let bytes: Vec<u8> = self.read_bytes(path).await.map_err(|error: TaskError| {
            TaskError::Storage(format!("no se pudo leer el adjunto {path}: {error}"))
        })?;
        Ok(STANDARD.encode(bytes))
    }

    async fn read_bytes(&self, path: &str) -> Result<Vec<u8>, TaskError> {
        let full: PathBuf = self.resolve(path)?;
        tokio::fs::read(&full).await.map_err(|e| {
            if e.kind() == std::io::ErrorKind::NotFound {
                TaskError::NotFound(format!("adjunto {path}"))
            } else {
                TaskError::Storage(format!("no se pudo leer el adjunto {path}: {e}"))
            }
        })
    }

    async fn delete(&self, path: &str) -> Result<(), TaskError> {
        let full: PathBuf = self.resolve(path)?;
        match tokio::fs::remove_file(&full).await {
            Ok(()) => Ok(()),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(e) => Err(io_error("no se pudo borrar el adjunto", e)),
        }
    }

    async fn delete_task_dir(&self, task_id: TaskId) -> Result<(), TaskError> {
        let dir: PathBuf = self.base_dir.join(task_id.to_string());
        match tokio::fs::remove_dir_all(&dir).await {
            Ok(()) => Ok(()),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(e) => Err(io_error("no se pudo borrar el directorio de adjuntos", e)),
        }
    }
}
