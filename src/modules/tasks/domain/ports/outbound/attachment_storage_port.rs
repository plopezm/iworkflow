use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::domain::models::task::TaskId;

/// Almacén de los ficheros de imagen adjuntos a las tareas. Las rutas son siempre relativas
/// al directorio base del almacén.
#[async_trait::async_trait]
pub trait AttachmentStoragePort: Send + Sync {
    /// Guarda la imagen (base64 en `image.data`) para la tarea y devuelve su ruta relativa.
    async fn save(
        &self,
        task_id: TaskId,
        attachment_id: &str,
        image: &ImageAttachment,
    ) -> Result<String, TaskError>;
    /// Lee el fichero y lo devuelve codificado en base64.
    async fn read(&self, path: &str) -> Result<String, TaskError>;
    /// Borra un fichero; no falla si ya no existe.
    async fn delete(&self, path: &str) -> Result<(), TaskError>;
    /// Borra el directorio de adjuntos de la tarea; no falla si no existe.
    async fn delete_task_dir(&self, task_id: TaskId) -> Result<(), TaskError>;
}
