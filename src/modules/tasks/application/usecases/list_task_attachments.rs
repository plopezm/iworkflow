use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;

/// Devuelve los metadatos de las imágenes adjuntas de una tarea, ordenados por posición. No
/// toca el disco: el contenido se sirve aparte (`GetTaskAttachment`).
pub struct ListTaskAttachments {
    attachments: Arc<dyn TaskAttachmentRepositoryPort>,
}

impl ListTaskAttachments {
    pub fn new(attachments: Arc<dyn TaskAttachmentRepositoryPort>) -> Self {
        Self { attachments }
    }

    pub async fn execute(&self, id: TaskId) -> Result<Vec<StoredAttachment>, TaskError> {
        self.attachments.list_by_task(id).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::MockTaskAttachmentRepository;

    fn stored(name: &str) -> StoredAttachment {
        StoredAttachment {
            id: format!("id-{name}"),
            name: name.to_string(),
            mime_type: "image/png".to_string(),
            path: format!("t/{name}"),
            size_bytes: 5,
        }
    }

    #[tokio::test]
    async fn returns_metadata_in_order() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .times(1)
            .returning(|_| Ok(vec![stored("a.png"), stored("b.png")]));

        let result: Vec<StoredAttachment> = ListTaskAttachments::new(Arc::new(attachments))
            .execute(TaskId::new())
            .await
            .expect("lista");

        assert_eq!(result, vec![stored("a.png"), stored("b.png")]);
    }

    #[tokio::test]
    async fn repository_failure_is_propagated() {
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .returning(|_| Err(TaskError::Repository("boom".to_string())));

        let result: Result<Vec<StoredAttachment>, TaskError> =
            ListTaskAttachments::new(Arc::new(attachments))
                .execute(TaskId::new())
                .await;

        assert_eq!(result, Err(TaskError::Repository("boom".to_string())));
    }
}
