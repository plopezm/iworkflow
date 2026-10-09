use std::sync::Arc;

use uuid::Uuid;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::image_attachment::{
    ImageAttachment, MAX_IMAGES_PER_MESSAGE,
};
use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
use crate::modules::tasks::domain::models::task::{NewTask, Task, TaskId};
use crate::modules::tasks::domain::models::task_priority::TaskPriority;
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;

/// Edita titulo, prompt y prioridad de una tarea en `Backlog` (misma validacion que al crearla)
/// quita las imágenes indicadas y añade imágenes nuevas a las ya adjuntas.
pub struct UpdateTask {
    lifecycle: Arc<TaskLifecycle>,
    attachments: Arc<dyn TaskAttachmentRepositoryPort>,
    storage: Arc<dyn AttachmentStoragePort>,
}

impl UpdateTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        attachments: Arc<dyn TaskAttachmentRepositoryPort>,
        storage: Arc<dyn AttachmentStoragePort>,
    ) -> Self {
        Self {
            lifecycle,
            attachments,
            storage,
        }
    }

    /// Guarda los ficheros nuevos y después sus metadatos. Si algo falla, borra solo lo recién escrito.
    async fn store_new_images(
        &self,
        task_id: TaskId,
        images: &[ImageAttachment],
    ) -> Result<(), TaskError> {
        let mut saved_paths: Vec<String> = Vec::with_capacity(images.len());
        let result: Result<(), TaskError> = self
            .save_and_register(task_id, images, &mut saved_paths)
            .await;
        if result.is_err() {
            for path in &saved_paths {
                if let Err(cleanup_error) = self.storage.delete(path).await {
                    tracing::warn!(
                        %task_id,
                        %path,
                        %cleanup_error,
                        "no se pudo borrar un adjunto escrito tras un fallo"
                    );
                }
            }
        }
        result
    }

    async fn save_and_register(
        &self,
        task_id: TaskId,
        images: &[ImageAttachment],
        saved_paths: &mut Vec<String>,
    ) -> Result<(), TaskError> {
        let mut stored: Vec<StoredAttachment> = Vec::with_capacity(images.len());
        for image in images {
            let id: String = Uuid::new_v4().to_string();
            let path: String = self.storage.save(task_id, &id, image).await?;
            saved_paths.push(path.clone());
            stored.push(StoredAttachment {
                id,
                name: image.name.clone(),
                mime_type: image.mime_type.clone(),
                path,
                size_bytes: image.decoded_len() as u64,
            });
        }
        self.attachments.insert_many(task_id, &stored).await
    }

    /// Borra los metadatos y, en modo mejor esfuerzo, los ficheros de las imágenes quitadas.
    async fn remove_attachments(&self, task_id: TaskId, ids: &[String]) -> Result<(), TaskError> {
        let removed: Vec<StoredAttachment> = self.attachments.remove(task_id, ids).await?;
        for attachment in &removed {
            if let Err(error) = self.storage.delete(&attachment.path).await {
                tracing::warn!(
                    %task_id,
                    path = %attachment.path,
                    %error,
                    "no se pudo borrar el fichero de un adjunto quitado"
                );
            }
        }
        Ok(())
    }

    pub async fn execute(
        &self,
        id: TaskId,
        title: String,
        prompt: String,
        priority: TaskPriority,
        images: Vec<ImageAttachment>,
        remove_attachment_ids: Vec<String>,
    ) -> Result<Task, TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_edit() {
            return Err(invalid_state("editar", task.status));
        }
        let validated: NewTask = NewTask {
            workspace_id: task.workspace_id.clone(),
            title,
            prompt,
            priority,
            images: Vec::new(),
            start_now: false,
        }
        .validated()?;
        let images: Vec<ImageAttachment> = ImageAttachment::validate_all(images)?;
        let mut removable_ids: Vec<String> = Vec::new();
        if !images.is_empty() || !remove_attachment_ids.is_empty() {
            let existing: Vec<StoredAttachment> = self.attachments.list_by_task(id).await?;
            removable_ids = existing
                .iter()
                .filter(|attachment: &&StoredAttachment| {
                    remove_attachment_ids.contains(&attachment.id)
                })
                .map(|attachment: &StoredAttachment| attachment.id.clone())
                .collect();
            let remaining: usize = existing.len() - removable_ids.len();
            if remaining + images.len() > MAX_IMAGES_PER_MESSAGE {
                return Err(TaskError::InvalidAttachment(format!(
                    "máximo {MAX_IMAGES_PER_MESSAGE} imágenes por tarea"
                )));
            }
        }

        let updated: bool = self
            .lifecycle
            .repository()
            .update_details_if_backlog(
                id,
                validated.title.clone(),
                validated.prompt.clone(),
                validated.priority,
            )
            .await?;
        if !updated {
            return Err(self.lifecycle.conflict(id, "editar").await);
        }
        if !removable_ids.is_empty() {
            self.remove_attachments(id, &removable_ids).await?;
        }
        if !images.is_empty() {
            self.store_new_images(id, &images).await?;
        }
        self.lifecycle.notify_changed(id).await?;

        Ok(Task {
            title: validated.title,
            prompt: validated.prompt,
            priority: validated.priority,
            ..task
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::{
        MockAttachmentStorage, MockTaskAttachmentRepository, MockTaskRepository,
    };

    fn use_case_with(
        lifecycle: Arc<TaskLifecycle>,
        attachments: MockTaskAttachmentRepository,
        storage: MockAttachmentStorage,
    ) -> UpdateTask {
        UpdateTask::new(lifecycle, Arc::new(attachments), Arc::new(storage))
    }

    /// Use case whose attachment ports fail the test if they are called.
    fn use_case(lifecycle: Arc<TaskLifecycle>) -> UpdateTask {
        use_case_with(
            lifecycle,
            MockTaskAttachmentRepository::new(),
            MockAttachmentStorage::new(),
        )
    }

    fn image(name: &str) -> ImageAttachment {
        ImageAttachment {
            name: name.to_string(),
            mime_type: "image/png".to_string(),
            data: "aGVsbG8=".to_string(),
        }
    }

    fn stored(name: &str) -> StoredAttachment {
        StoredAttachment {
            id: name.to_string(),
            name: name.to_string(),
            mime_type: "image/png".to_string(),
            path: format!("t/{name}"),
            size_bytes: 5,
        }
    }

    fn lifecycle_accepting_update(task: Task, recorded: &Recorded) -> Arc<TaskLifecycle> {
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), recorded);
        repository
            .expect_update_details_if_backlog()
            .times(1)
            .returning(|_, _, _, _| Ok(true));
        test_support::lifecycle_with(repository, recorded)
    }

    #[tokio::test]
    async fn updates_title_and_prompt_when_backlog() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();

        let updated: Task = use_case(lifecycle_accepting_update(task, &recorded))
            .execute(
                id,
                "Nuevo titulo".to_string(),
                "Nuevo prompt".to_string(),
                TaskPriority::default(),
                Vec::new(),
                Vec::new(),
            )
            .await
            .expect("editada");

        assert_eq!(updated.title, "Nuevo titulo");
        assert_eq!(updated.prompt, "Nuevo prompt");
    }

    #[tokio::test]
    async fn updates_priority_and_persists_it_through_the_port() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_update_details_if_backlog()
            .times(1)
            .withf(|_, _, _, priority: &TaskPriority| *priority == TaskPriority::Urgent)
            .returning(|_, _, _, _| Ok(true));

        let updated: Task = use_case(test_support::lifecycle_with(repository, &recorded))
            .execute(
                id,
                "t".to_string(),
                "p".to_string(),
                TaskPriority::Urgent,
                Vec::new(),
                Vec::new(),
            )
            .await
            .expect("editada");

        assert_eq!(updated.priority, TaskPriority::Urgent);
    }

    #[tokio::test]
    async fn derives_title_from_prompt_when_blank() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();

        let updated: Task = use_case(lifecycle_accepting_update(task, &recorded))
            .execute(
                id,
                "  ".to_string(),
                "Arregla el test\nmas".to_string(),
                TaskPriority::default(),
                Vec::new(),
                Vec::new(),
            )
            .await
            .expect("editada");

        assert_eq!(updated.title, "Arregla el test");
    }

    #[tokio::test]
    async fn rejects_task_not_in_backlog() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let result: Result<Task, TaskError> =
            use_case(test_support::lifecycle(Some(task), &Recorded::default()))
                .execute(
                    id,
                    "t".to_string(),
                    "p".to_string(),
                    TaskPriority::default(),
                    Vec::new(),
                    Vec::new(),
                )
                .await;

        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }

    #[tokio::test]
    async fn rejects_when_task_left_backlog_concurrently() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_update_details_if_backlog()
            .times(1)
            .returning(|_, _, _, _| Ok(false));

        let result: Result<Task, TaskError> =
            use_case(test_support::lifecycle_with(repository, &recorded))
                .execute(
                    id,
                    "t".to_string(),
                    "p".to_string(),
                    TaskPriority::default(),
                    Vec::new(),
                    Vec::new(),
                )
                .await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn rejects_empty_prompt() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let result: Result<Task, TaskError> =
            use_case(test_support::lifecycle(Some(task), &Recorded::default()))
                .execute(
                    id,
                    "t".to_string(),
                    "   ".to_string(),
                    TaskPriority::default(),
                    Vec::new(),
                    Vec::new(),
                )
                .await;

        assert_eq!(result, Err(TaskError::EmptyPrompt));
    }

    #[tokio::test]
    async fn stores_new_images_after_updating_details() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .times(1)
            .returning(|_| Ok(vec![stored("old.png")]));
        attachments
            .expect_insert_many()
            .times(1)
            .withf(|_, items: &[StoredAttachment]| items.len() == 2 && items[0].path == "t/a.png")
            .returning(|_, _| Ok(()));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_save()
            .times(2)
            .returning(|_, _, image: &ImageAttachment| Ok(format!("t/{}", image.name)));

        use_case_with(
            lifecycle_accepting_update(task, &recorded),
            attachments,
            storage,
        )
        .execute(
            id,
            "t".to_string(),
            "p".to_string(),
            TaskPriority::default(),
            vec![image("a.png"), image("b.png")],
            Vec::new(),
        )
        .await
        .expect("editada");
    }

    #[tokio::test]
    async fn rejects_images_beyond_the_per_task_limit() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .times(1)
            .returning(|_| Ok(vec![stored("1"), stored("2"), stored("3"), stored("4")]));

        let result: Result<Task, TaskError> = use_case_with(
            test_support::lifecycle(Some(task), &Recorded::default()),
            attachments,
            MockAttachmentStorage::new(),
        )
        .execute(
            id,
            "t".to_string(),
            "p".to_string(),
            TaskPriority::default(),
            vec![image("a.png"), image("b.png")],
            Vec::new(),
        )
        .await;

        assert!(matches!(result, Err(TaskError::InvalidAttachment(_))));
    }

    #[tokio::test]
    async fn rejects_invalid_images() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let mut bad: ImageAttachment = image("a.txt");
        bad.mime_type = "text/plain".to_string();

        let result: Result<Task, TaskError> =
            use_case(test_support::lifecycle(Some(task), &Recorded::default()))
                .execute(
                    id,
                    "t".to_string(),
                    "p".to_string(),
                    TaskPriority::default(),
                    vec![bad],
                    Vec::new(),
                )
                .await;

        assert!(matches!(result, Err(TaskError::InvalidAttachment(_))));
    }

    #[tokio::test]
    async fn deletes_only_new_files_when_registering_them_fails() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .returning(|_| Ok(vec![stored("old.png")]));
        attachments
            .expect_insert_many()
            .times(1)
            .returning(|_, _| Err(TaskError::Repository("boom".to_string())));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_save()
            .times(2)
            .returning(|_, _, image: &ImageAttachment| Ok(format!("t/{}", image.name)));
        storage
            .expect_delete()
            .times(2)
            .withf(|path: &str| path == "t/a.png" || path == "t/b.png")
            .returning(|_| Err(TaskError::Repository("cleanup".to_string())));

        let result: Result<Task, TaskError> = use_case_with(
            lifecycle_accepting_update(task, &recorded),
            attachments,
            storage,
        )
        .execute(
            id,
            "t".to_string(),
            "p".to_string(),
            TaskPriority::default(),
            vec![image("a.png"), image("b.png")],
            Vec::new(),
        )
        .await;

        assert_eq!(result, Err(TaskError::Repository("boom".to_string())));
    }

    #[tokio::test]
    async fn removes_attachments_and_deletes_their_files() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .times(1)
            .returning(|_| Ok(vec![stored("a.png"), stored("b.png")]));
        attachments
            .expect_remove()
            .times(1)
            .withf(|_, ids: &[String]| ids == ["a.png".to_string()])
            .returning(|_, _| Ok(vec![stored("a.png")]));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_delete()
            .times(1)
            .withf(|path: &str| path == "t/a.png")
            .returning(|_| Ok(()));

        use_case_with(
            lifecycle_accepting_update(task, &recorded),
            attachments,
            storage,
        )
        .execute(
            id,
            "t".to_string(),
            "p".to_string(),
            TaskPriority::default(),
            Vec::new(),
            vec!["a.png".to_string(), "unknown".to_string()],
        )
        .await
        .expect("editada");
    }

    #[tokio::test]
    async fn file_deletion_failure_does_not_fail_the_update() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments
            .expect_list_by_task()
            .returning(|_| Ok(vec![stored("a.png")]));
        attachments
            .expect_remove()
            .times(1)
            .returning(|_, _| Ok(vec![stored("a.png")]));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_delete()
            .times(1)
            .returning(|_| Err(TaskError::Repository("disk".to_string())));

        let result: Result<Task, TaskError> = use_case_with(
            lifecycle_accepting_update(task, &recorded),
            attachments,
            storage,
        )
        .execute(
            id,
            "t".to_string(),
            "p".to_string(),
            TaskPriority::default(),
            Vec::new(),
            vec!["a.png".to_string()],
        )
        .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn limit_counts_attachments_after_removal() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments.expect_list_by_task().returning(|_| {
            Ok(vec![
                stored("1"),
                stored("2"),
                stored("3"),
                stored("4"),
                stored("5"),
            ])
        });
        attachments
            .expect_remove()
            .times(1)
            .returning(|_, _| Ok(vec![stored("1"), stored("2")]));
        attachments
            .expect_insert_many()
            .times(1)
            .returning(|_, _| Ok(()));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage.expect_delete().returning(|_| Ok(()));
        storage
            .expect_save()
            .times(2)
            .returning(|_, _, image: &ImageAttachment| Ok(format!("t/{}", image.name)));

        let result: Result<Task, TaskError> = use_case_with(
            lifecycle_accepting_update(task, &recorded),
            attachments,
            storage,
        )
        .execute(
            id,
            "t".to_string(),
            "p".to_string(),
            TaskPriority::default(),
            vec![image("a.png"), image("b.png")],
            vec!["1".to_string(), "2".to_string()],
        )
        .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn rejects_when_remaining_plus_new_exceed_the_limit() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let mut attachments: MockTaskAttachmentRepository = MockTaskAttachmentRepository::new();
        attachments.expect_list_by_task().returning(|_| {
            Ok(vec![
                stored("1"),
                stored("2"),
                stored("3"),
                stored("4"),
                stored("5"),
            ])
        });

        let result: Result<Task, TaskError> = use_case_with(
            test_support::lifecycle(Some(task), &Recorded::default()),
            attachments,
            MockAttachmentStorage::new(),
        )
        .execute(
            id,
            "t".to_string(),
            "p".to_string(),
            TaskPriority::default(),
            vec![image("a.png"), image("b.png")],
            vec!["1".to_string()],
        )
        .await;

        assert!(matches!(result, Err(TaskError::InvalidAttachment(_))));
    }

    #[tokio::test]
    async fn rejects_removal_when_task_is_not_editable() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;

        let result: Result<Task, TaskError> =
            use_case(test_support::lifecycle(Some(task), &Recorded::default()))
                .execute(
                    id,
                    "t".to_string(),
                    "p".to_string(),
                    TaskPriority::default(),
                    Vec::new(),
                    vec!["a.png".to_string()],
                )
                .await;

        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
