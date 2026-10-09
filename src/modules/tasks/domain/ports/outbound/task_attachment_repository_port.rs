use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::domain::models::task::TaskId;

#[async_trait::async_trait]
pub trait TaskAttachmentRepositoryPort: Send + Sync {
    /// Guarda las imágenes de una tarea, conservando el orden recibido, en una única
    /// transacción (todas o ninguna).
    async fn insert_many(
        &self,
        task_id: TaskId,
        images: &[ImageAttachment],
    ) -> Result<(), TaskError>;
    /// Imágenes de la tarea ordenadas por posición.
    async fn list_by_task(&self, task_id: TaskId) -> Result<Vec<ImageAttachment>, TaskError>;
}
