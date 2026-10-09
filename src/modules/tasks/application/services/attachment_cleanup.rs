use std::sync::Arc;

use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;

/// Limpieza de adjuntos de una tarea en modo "mejor esfuerzo": los fallos solo se registran,
/// nunca se propagan al caso de uso que la invoca.
pub struct AttachmentCleanup {
    attachments: Arc<dyn TaskAttachmentRepositoryPort>,
    storage: Arc<dyn AttachmentStoragePort>,
}

impl AttachmentCleanup {
    pub fn new(
        attachments: Arc<dyn TaskAttachmentRepositoryPort>,
        storage: Arc<dyn AttachmentStoragePort>,
    ) -> Self {
        Self {
            attachments,
            storage,
        }
    }

    /// Borra los metadatos y el directorio de ficheros de la tarea.
    pub async fn cleanup_task(&self, task_id: TaskId) {
        if let Err(error) = self.attachments.delete_by_task(task_id).await {
            tracing::warn!(%task_id, %error, "no se pudieron borrar los adjuntos de la tarea de la base de datos");
        }
        if let Err(error) = self.storage.delete_task_dir(task_id).await {
            tracing::warn!(%task_id, %error, "no se pudieron borrar los ficheros de adjuntos de la tarea");
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::errors::task_error::TaskError;
    use crate::modules::tasks::mocks::{MockAttachmentStorage, MockTaskAttachmentRepository};

    #[tokio::test]
    async fn removes_rows_and_directory() {
        let id: TaskId = TaskId::new();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_delete_by_task()
            .withf(move |task_id: &TaskId| *task_id == id)
            .times(1)
            .returning(|_| Ok(()));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_delete_task_dir()
            .withf(move |task_id: &TaskId| *task_id == id)
            .times(1)
            .returning(|_| Ok(()));

        AttachmentCleanup::new(Arc::new(attachments), Arc::new(storage))
            .cleanup_task(id)
            .await;
    }

    #[tokio::test]
    async fn repository_failure_still_deletes_the_directory() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_delete_by_task()
            .times(1)
            .returning(|_| Err(TaskError::Repository("boom".to_string())));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_delete_task_dir()
            .times(1)
            .returning(|_| Err(TaskError::Repository("disk".to_string())));

        AttachmentCleanup::new(Arc::new(attachments), Arc::new(storage))
            .cleanup_task(TaskId::new())
            .await;
    }
}
