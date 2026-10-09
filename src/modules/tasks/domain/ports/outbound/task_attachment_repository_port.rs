use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
use crate::modules::tasks::domain::models::task::TaskId;

#[async_trait::async_trait]
pub trait TaskAttachmentRepositoryPort: Send + Sync {
    /// Guarda los metadatos de las imágenes de una tarea, conservando el orden recibido, en
    /// una única transacción (todas o ninguna).
    async fn insert_many(
        &self,
        task_id: TaskId,
        attachments: &[StoredAttachment],
    ) -> Result<(), TaskError>;
    /// Metadatos de las imágenes de la tarea ordenados por posición.
    async fn list_by_task(&self, task_id: TaskId) -> Result<Vec<StoredAttachment>, TaskError>;
}
