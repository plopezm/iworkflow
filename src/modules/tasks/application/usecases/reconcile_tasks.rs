use std::path::Path;
use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskStatus};
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

const RECONCILE_MESSAGE: &str = "el servidor se reinició mientras la tarea estaba activa";

/// Al arrancar, ningún agente sigue vivo: las tareas activas pasan a `Interrupted`.
pub struct ReconcileTasks {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
}

impl ReconcileTasks {
    pub fn new(lifecycle: Arc<TaskLifecycle>, worktrees: Arc<dyn WorktreePort>) -> Self {
        Self {
            lifecycle,
            worktrees,
        }
    }

    async fn message_for(&self, task: &Task) -> String {
        let path: &Path = &task.worktree_path;
        if path.as_os_str().is_empty() || self.worktrees.exists(path).await {
            return RECONCILE_MESSAGE.to_string();
        }
        format!(
            "{RECONCILE_MESSAGE}; el worktree ya no existe en {} y la tarea solo se puede descartar",
            path.display()
        )
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
            let message: String = self.message_for(task).await;
            self.lifecycle
                .transition(task.id, TaskStatus::Interrupted, Some(message))
                .await?;
        }
        Ok(active.len())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::{MockTaskRepository, MockWorktree};

    fn lifecycle_for(tasks: Vec<Task>, recorded: &Recorded) -> Arc<TaskLifecycle> {
        let mut repository: MockTaskRepository = test_support::repository_with(None, recorded);
        repository
            .expect_list_by_status()
            .returning(move |_| Ok(tasks.clone()));
        Arc::new(TaskLifecycle::new(
            Arc::new(repository),
            Arc::new(test_support::publisher(recorded)),
            Arc::new(test_support::workspaces()),
        ))
    }

    fn worktrees_existing(exists: bool) -> Arc<dyn WorktreePort> {
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_exists().returning(move |_| exists);
        Arc::new(worktrees)
    }

    async fn reconcile_error(worktree_exists: bool) -> Option<String> {
        let recorded: Recorded = Recorded::default();
        let lifecycle: Arc<TaskLifecycle> =
            lifecycle_for(vec![test_support::task(TaskStatus::Running)], &recorded);
        ReconcileTasks::new(lifecycle, worktrees_existing(worktree_exists))
            .execute()
            .await
            .expect("ok");
        recorded.last_error()
    }

    #[tokio::test]
    async fn missing_worktree_gets_discard_only_message() {
        let message: String = reconcile_error(false).await.expect("mensaje");
        assert!(message.contains("ya no existe"), "{message}");
        assert!(message.contains(test_support::WORKTREE), "{message}");
    }

    #[tokio::test]
    async fn existing_worktree_keeps_restart_message() {
        let message: Option<String> = reconcile_error(true).await;
        assert_eq!(message.as_deref(), Some(RECONCILE_MESSAGE));
    }

    #[tokio::test]
    async fn marks_active_tasks_as_interrupted() {
        let recorded: Recorded = Recorded::default();
        let lifecycle: Arc<TaskLifecycle> = lifecycle_for(
            vec![
                test_support::task(TaskStatus::Running),
                test_support::task(TaskStatus::Idle),
            ],
            &recorded,
        );

        let count: usize = ReconcileTasks::new(lifecycle, worktrees_existing(true))
            .execute()
            .await
            .expect("ok");

        assert_eq!(count, 2);
        assert_eq!(
            recorded.statuses(),
            vec![TaskStatus::Interrupted, TaskStatus::Interrupted]
        );
    }
}
