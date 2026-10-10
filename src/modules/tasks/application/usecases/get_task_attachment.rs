use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_attachment_content::TaskAttachmentContent;
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;

/// Devuelve el contenido de una imagen adjunta. La ruta en disco sale siempre de los
/// metadatos guardados de la tarea (nunca del identificador recibido), y un adjunto de otra
/// tarea cuenta como inexistente.
pub struct GetTaskAttachment {
    attachments: Arc<dyn TaskAttachmentRepositoryPort>,
    storage: Arc<dyn AttachmentStoragePort>,
}

impl GetTaskAttachment {
    pub fn new(
        attachments: Arc<dyn TaskAttachmentRepositoryPort>,
        storage: Arc<dyn AttachmentStoragePort>,
    ) -> Self {
        Self {
            attachments,
            storage,
        }
    }

    pub async fn execute(
        &self,
        task_id: TaskId,
        attachment_id: &str,
    ) -> Result<TaskAttachmentContent, TaskError> {
        let stored: Vec<StoredAttachment> = self.attachments.list_by_task(task_id).await?;
        let attachment: StoredAttachment = stored
            .into_iter()
            .find(|a: &StoredAttachment| a.id == attachment_id)
            .ok_or_else(|| TaskError::NotFound(format!("adjunto {attachment_id}")))?;
        let bytes: Vec<u8> = self.storage.read_bytes(&attachment.path).await?;
        Ok(TaskAttachmentContent {
            mime_type: attachment.mime_type,
            bytes,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::{MockAttachmentStorage, MockTaskAttachmentRepository};

    fn stored(id: &str) -> StoredAttachment {
        StoredAttachment {
            id: id.to_string(),
            name: format!("{id}.png"),
            mime_type: "image/png".to_string(),
            path: format!("t/{id}.png"),
            size_bytes: 3,
        }
    }

    fn use_case(
        attachments: MockTaskAttachmentRepository,
        storage: MockAttachmentStorage,
    ) -> GetTaskAttachment {
        GetTaskAttachment::new(Arc::new(attachments), Arc::new(storage))
    }

    #[tokio::test]
    async fn returns_bytes_and_mime_of_the_matching_attachment() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .returning(|_| Ok(vec![stored("a"), stored("b")]));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_read_bytes()
            .withf(|path: &str| path == "t/b.png")
            .times(1)
            .returning(|_| Ok(vec![1, 2, 3]));

        let content: TaskAttachmentContent = use_case(attachments, storage)
            .execute(TaskId::new(), "b")
            .await
            .expect("contenido");

        assert_eq!(
            content,
            TaskAttachmentContent {
                mime_type: "image/png".to_string(),
                bytes: vec![1, 2, 3],
            }
        );
    }

    #[tokio::test]
    async fn unknown_or_foreign_attachment_is_not_found_without_touching_disk() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        // La tarea consultada no tiene el adjunto (pertenece a otra tarea).
        attachments.expect_list_by_task().returning(|_| Ok(vec![]));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage.expect_read_bytes().never();

        let result: Result<TaskAttachmentContent, TaskError> = use_case(attachments, storage)
            .execute(TaskId::new(), "b")
            .await;

        assert!(matches!(result, Err(TaskError::NotFound(_))));
    }

    #[tokio::test]
    async fn missing_file_and_repository_errors_are_propagated() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .returning(|_| Ok(vec![stored("a")]));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_read_bytes()
            .returning(|_| Err(TaskError::NotFound("adjunto".to_string())));
        let missing: Result<TaskAttachmentContent, TaskError> = use_case(attachments, storage)
            .execute(TaskId::new(), "a")
            .await;

        let mut failing: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        failing
            .expect_list_by_task()
            .returning(|_| Err(TaskError::Repository("boom".to_string())));
        let broken: Result<TaskAttachmentContent, TaskError> =
            use_case(failing, MockAttachmentStorage::new())
                .execute(TaskId::new(), "a")
                .await;

        assert!(matches!(missing, Err(TaskError::NotFound(_))));
        assert_eq!(broken, Err(TaskError::Repository("boom".to_string())));
    }
}
