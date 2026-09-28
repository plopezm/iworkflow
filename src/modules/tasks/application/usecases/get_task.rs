use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};

/// Obtiene una tarea por id; `NotFound` si no existe.
pub struct GetTask {
    lifecycle: Arc<TaskLifecycle>,
}

impl GetTask {
    pub fn new(lifecycle: Arc<TaskLifecycle>) -> Self {
        Self { lifecycle }
    }

    pub async fn execute(&self, id: TaskId) -> Result<Task, TaskError> {
        self.lifecycle.load(id).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};

    #[tokio::test]
    async fn missing_task_is_not_found() {
        let id: TaskId = TaskId::new();
        let result: Result<Task, TaskError> =
            GetTask::new(test_support::lifecycle(None, &Recorded::default()))
                .execute(id)
                .await;
        assert_eq!(result, Err(TaskError::NotFound(id.to_string())));
    }
}
