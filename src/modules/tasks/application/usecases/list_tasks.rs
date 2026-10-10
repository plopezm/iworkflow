use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::Task;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;

/// Lista las tareas no finalizadas (más recientes primero), opcionalmente de un workspace.
pub struct ListTasks {
    repository: Arc<dyn TaskRepositoryPort>,
}

impl ListTasks {
    pub fn new(repository: Arc<dyn TaskRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError> {
        let workspace_id: Option<String> = workspace_id
            .map(|id: String| id.trim().to_string())
            .filter(|id: &String| !id.is_empty());
        self.repository.list_active(workspace_id).await
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::tasks::mocks::MockTaskRepository;

    #[tokio::test]
    async fn blank_filter_lists_all_active() {
        let mut repository: MockTaskRepository = MockTaskRepository::new();
        repository
            .expect_list_active()
            .with(eq(None))
            .times(1)
            .returning(|_| Ok(vec![]));

        let tasks: Vec<Task> = ListTasks::new(Arc::new(repository))
            .execute(Some("  ".to_string()))
            .await
            .expect("lista");

        assert!(tasks.is_empty());
    }
}
