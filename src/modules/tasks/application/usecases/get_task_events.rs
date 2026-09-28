use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_event::TaskEvent;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;

/// Historial persistido de una tarea a partir de un `seq` (exclusivo).
pub struct GetTaskEvents {
    repository: Arc<dyn TaskRepositoryPort>,
}

impl GetTaskEvents {
    pub fn new(repository: Arc<dyn TaskRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(
        &self,
        id: TaskId,
        after_seq: Option<i64>,
    ) -> Result<Vec<TaskEvent>, TaskError> {
        self.repository.events(id, after_seq).await
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::tasks::mocks::MockTaskRepository;

    #[tokio::test]
    async fn delegates_cursor_to_repository() {
        let id: TaskId = TaskId::new();
        let mut repository: MockTaskRepository = MockTaskRepository::new();
        repository
            .expect_events()
            .with(eq(id), eq(Some(7)))
            .times(1)
            .returning(|_, _| Ok(vec![]));

        let events: Vec<TaskEvent> = GetTaskEvents::new(Arc::new(repository))
            .execute(id, Some(7))
            .await
            .expect("eventos");

        assert!(events.is_empty());
    }
}
