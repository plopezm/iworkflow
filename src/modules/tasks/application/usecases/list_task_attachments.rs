use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_attachment_content::TaskAttachmentContent;
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;

/// Devuelve las imágenes adjuntas de una tarea con su contenido en base64. Un fichero ilegible
/// se omite (con aviso en el log) sin hacer fallar el listado.
pub struct ListTaskAttachments {
    attachments: Arc<dyn TaskAttachmentRepositoryPort>,
    storage: Arc<dyn AttachmentStoragePort>,
}

impl ListTaskAttachments {
    pub fn new(
        attachments: Arc<dyn TaskAttachmentRepositoryPort>,
        storage: Arc<dyn AttachmentStoragePort>,
    ) -> Self {
        Self {
            attachments,
            storage,
        }
    }

    pub async fn execute(&self, id: TaskId) -> Result<Vec<TaskAttachmentContent>, TaskError> {
        let stored: Vec<StoredAttachment> = self.attachments.list_by_task(id).await?;
        let mut contents: Vec<TaskAttachmentContent> = Vec::with_capacity(stored.len());
        for attachment in stored {
            match self.storage.read(&attachment.path).await {
                Ok(data) => contents.push(TaskAttachmentContent {
                    id: attachment.id,
                    name: attachment.name,
                    mime_type: attachment.mime_type,
                    data,
                }),
                Err(error) => tracing::warn!(
                    task_id = %id,
                    attachment_id = %attachment.id,
                    %error,
                    "no se pudo leer un adjunto; se omite del listado"
                ),
            }
        }
        Ok(contents)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::{MockAttachmentStorage, MockTaskAttachmentRepository};

    fn stored(name: &str) -> StoredAttachment {
        StoredAttachment {
            id: format!("id-{name}"),
            name: name.to_string(),
            mime_type: "image/png".to_string(),
            path: format!("t/{name}"),
            size_bytes: 5,
        }
    }

    fn use_case(
        attachments: MockTaskAttachmentRepository,
        storage: MockAttachmentStorage,
    ) -> ListTaskAttachments {
        ListTaskAttachments::new(Arc::new(attachments), Arc::new(storage))
    }

    #[tokio::test]
    async fn returns_attachments_with_their_content() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .times(1)
            .returning(|_| Ok(vec![stored("a.png"), stored("b.png")]));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_read()
            .times(2)
            .returning(|path: &str| Ok(format!("data:{path}")));

        let result: Vec<TaskAttachmentContent> = use_case(attachments, storage)
            .execute(TaskId::new())
            .await
            .expect("lista");

        assert_eq!(
            result,
            vec![
                TaskAttachmentContent {
                    id: "id-a.png".to_string(),
                    name: "a.png".to_string(),
                    mime_type: "image/png".to_string(),
                    data: "data:t/a.png".to_string(),
                },
                TaskAttachmentContent {
                    id: "id-b.png".to_string(),
                    name: "b.png".to_string(),
                    mime_type: "image/png".to_string(),
                    data: "data:t/b.png".to_string(),
                },
            ]
        );
    }

    #[tokio::test]
    async fn skips_unreadable_files_without_failing() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .returning(|_| Ok(vec![stored("a.png"), stored("b.png")]));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage.expect_read().returning(|path: &str| {
            if path == "t/a.png" {
                Err(TaskError::Repository("missing".to_string()))
            } else {
                Ok("ok".to_string())
            }
        });

        let result: Vec<TaskAttachmentContent> = use_case(attachments, storage)
            .execute(TaskId::new())
            .await
            .expect("lista");

        assert_eq!(result.len(), 1);
        assert_eq!(result[0].id, "id-b.png");
    }

    #[tokio::test]
    async fn repository_failure_is_propagated() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .returning(|_| Err(TaskError::Repository("boom".to_string())));

        let result: Result<Vec<TaskAttachmentContent>, TaskError> =
            use_case(attachments, MockAttachmentStorage::new())
                .execute(TaskId::new())
                .await;

        assert_eq!(result, Err(TaskError::Repository("boom".to_string())));
    }
}
