use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::Task;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;

const MIN_LIMIT: i64 = 1;
const MAX_LIMIT: i64 = 100;

/// Lista una página de tareas finalizadas (más recientes primero), opcionalmente de un workspace.
pub struct ListDoneTasks {
    repository: Arc<dyn TaskRepositoryPort>,
}

impl ListDoneTasks {
    pub fn new(repository: Arc<dyn TaskRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(
        &self,
        workspace_id: Option<String>,
        limit: i64,
        offset: i64,
    ) -> Result<Vec<Task>, TaskError> {
        let workspace_id: Option<String> = workspace_id
            .map(|id: String| id.trim().to_string())
            .filter(|id: &String| !id.is_empty());
        self.repository
            .list_done_page(
                workspace_id,
                limit.clamp(MIN_LIMIT, MAX_LIMIT),
                offset.max(0),
            )
            .await
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::tasks::mocks::MockTaskRepository;

    async fn run(
        workspace: Option<&str>,
        limit: i64,
        offset: i64,
        expected: (Option<&str>, i64, i64),
    ) {
        let mut repository: MockTaskRepository = MockTaskRepository::new();
        repository
            .expect_list_done_page()
            .with(
                eq(expected.0.map(str::to_string)),
                eq(expected.1),
                eq(expected.2),
            )
            .times(1)
            .returning(|_, _, _| Ok(vec![]));

        let tasks: Vec<Task> = ListDoneTasks::new(Arc::new(repository))
            .execute(workspace.map(str::to_string), limit, offset)
            .await
            .expect("lista");

        assert!(tasks.is_empty());
    }

    #[tokio::test]
    async fn passes_valid_values_through() {
        run(Some("ws"), 50, 10, (Some("ws"), 50, 10)).await;
    }

    #[tokio::test]
    async fn clamps_limit_and_offset() {
        run(None, 0, -5, (None, 1, 0)).await;
        run(None, 1000, 3, (None, 100, 3)).await;
    }

    #[tokio::test]
    async fn blank_workspace_means_all() {
        run(Some("  "), 20, 0, (None, 20, 0)).await;
    }
}
