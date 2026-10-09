use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;

/// Elimina físicamente una tarea en `Backlog` (los eventos y adjuntos asociados cascadean) y
/// borra, sin bloquear el borrado si falla, sus ficheros de imagen.
pub struct DeleteTask {
    lifecycle: Arc<TaskLifecycle>,
    storage: Arc<dyn AttachmentStoragePort>,
}

impl DeleteTask {
    pub fn new(lifecycle: Arc<TaskLifecycle>, storage: Arc<dyn AttachmentStoragePort>) -> Self {
        Self { lifecycle, storage }
    }

    pub async fn execute(&self, id: TaskId) -> Result<(), TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_delete() {
            return Err(invalid_state("eliminar", task.status));
        }
        let deleted: bool = self.lifecycle.repository().delete_if_backlog(id).await?;
        if !deleted {
            return Err(self.lifecycle.conflict(id, "eliminar").await);
        }
        if let Err(error) = self.storage.delete_task_dir(id).await {
            tracing::warn!(task_id = %id, %error, "no se pudieron borrar los adjuntos de la tarea");
        }
        self.lifecycle.notify_removed(id).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::mocks::{MockAttachmentStorage, MockTaskRepository};

    #[tokio::test]
    async fn deletes_task_when_backlog() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_delete_if_backlog()
            .times(1)
            .returning(|_| Ok(true));

        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_delete_task_dir()
            .withf(move |task_id: &TaskId| *task_id == id)
            .times(1)
            .returning(|_| Ok(()));

        DeleteTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(storage),
        )
        .execute(id)
        .await
        .expect("eliminada");
    }

    #[tokio::test]
    async fn storage_failure_does_not_block_deletion() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_delete_if_backlog()
            .times(1)
            .returning(|_| Ok(true));
        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage
            .expect_delete_task_dir()
            .times(1)
            .returning(|_| Err(TaskError::Storage("permiso denegado".to_string())));

        let result: Result<(), TaskError> = DeleteTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(storage),
        )
        .execute(id)
        .await;

        assert_eq!(result, Ok(()));
    }

    #[tokio::test]
    async fn rejects_when_task_left_backlog_concurrently() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_delete_if_backlog()
            .times(1)
            .returning(|_| Ok(false));

        let mut storage: MockAttachmentStorage = MockAttachmentStorage::new();
        storage.expect_delete_task_dir().never();

        let result: Result<(), TaskError> = DeleteTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(storage),
        )
        .execute(id)
        .await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn rejects_task_not_in_backlog() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let result: Result<(), TaskError> = DeleteTask::new(
            test_support::lifecycle(Some(task), &Recorded::default()),
            Arc::new(MockAttachmentStorage::new()),
        )
        .execute(id)
        .await;

        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }
}
