use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{NewTask, Task, TaskId};
use crate::modules::tasks::domain::models::task_priority::TaskPriority;

/// Edita titulo, prompt y prioridad de una tarea en `Backlog` (misma validacion que al crearla).
pub struct UpdateTask {
    lifecycle: Arc<TaskLifecycle>,
}

impl UpdateTask {
    pub fn new(lifecycle: Arc<TaskLifecycle>) -> Self {
        Self { lifecycle }
    }

    pub async fn execute(
        &self,
        id: TaskId,
        title: String,
        prompt: String,
        priority: TaskPriority,
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
            start_now: false,
        }
        .validated()?;

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
    use crate::modules::tasks::mocks::MockTaskRepository;

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

        let updated: Task = UpdateTask::new(lifecycle_accepting_update(task, &recorded))
            .execute(
                id,
                "Nuevo titulo".to_string(),
                "Nuevo prompt".to_string(),
                TaskPriority::default(),
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

        let updated: Task = UpdateTask::new(test_support::lifecycle_with(repository, &recorded))
            .execute(id, "t".to_string(), "p".to_string(), TaskPriority::Urgent)
            .await
            .expect("editada");

        assert_eq!(updated.priority, TaskPriority::Urgent);
    }

    #[tokio::test]
    async fn derives_title_from_prompt_when_blank() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();

        let updated: Task = UpdateTask::new(lifecycle_accepting_update(task, &recorded))
            .execute(
                id,
                "  ".to_string(),
                "Arregla el test\nmas".to_string(),
                TaskPriority::default(),
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
            UpdateTask::new(test_support::lifecycle(Some(task), &Recorded::default()))
                .execute(
                    id,
                    "t".to_string(),
                    "p".to_string(),
                    TaskPriority::default(),
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
            UpdateTask::new(test_support::lifecycle_with(repository, &recorded))
                .execute(
                    id,
                    "t".to_string(),
                    "p".to_string(),
                    TaskPriority::default(),
                )
                .await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn rejects_empty_prompt() {
        let task: Task = test_support::task(TaskStatus::Backlog);
        let id: TaskId = task.id;
        let result: Result<Task, TaskError> =
            UpdateTask::new(test_support::lifecycle(Some(task), &Recorded::default()))
                .execute(
                    id,
                    "t".to_string(),
                    "   ".to_string(),
                    TaskPriority::default(),
                )
                .await;

        assert_eq!(result, Err(TaskError::EmptyPrompt));
    }
}
