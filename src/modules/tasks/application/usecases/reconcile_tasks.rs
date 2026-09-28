use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskStatus};

const RECONCILE_MESSAGE: &str = "el servidor se reinició mientras la tarea estaba activa";

/// Al arrancar, ningún agente sigue vivo: las tareas activas pasan a `Interrupted`.
pub struct ReconcileTasks {
    lifecycle: Arc<TaskLifecycle>,
}

impl ReconcileTasks {
    pub fn new(lifecycle: Arc<TaskLifecycle>) -> Self {
        Self { lifecycle }
    }

    /// Devuelve cuántas tareas se han marcado como interrumpidas.
    pub async fn execute(&self) -> Result<usize, TaskError> {
        let active: Vec<Task> = self
            .lifecycle
            .repository()
            .list_by_status(&[
                TaskStatus::Starting,
                TaskStatus::Running,
                TaskStatus::Idle,
                TaskStatus::Finishing,
            ])
            .await?;
        for task in &active {
            self.lifecycle
                .transition(
                    task.id,
                    TaskStatus::Interrupted,
                    Some(RECONCILE_MESSAGE.to_string()),
                )
                .await?;
        }
        Ok(active.len())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::MockTaskRepository;

    #[tokio::test]
    async fn marks_active_tasks_as_interrupted() {
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository = test_support::repository_with(None, &recorded);
        repository.expect_list_by_status().returning(|_| {
            Ok(vec![
                test_support::task(TaskStatus::Running),
                test_support::task(TaskStatus::Idle),
            ])
        });
        let lifecycle: Arc<TaskLifecycle> = Arc::new(TaskLifecycle::new(
            Arc::new(repository),
            Arc::new(test_support::publisher(&recorded)),
            Arc::new(test_support::workspaces()),
        ));

        let count: usize = ReconcileTasks::new(lifecycle).execute().await.expect("ok");

        assert_eq!(count, 2);
        assert_eq!(
            recorded.statuses(),
            vec![TaskStatus::Interrupted, TaskStatus::Interrupted]
        );
    }
}
