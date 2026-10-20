use std::path::PathBuf;
use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskStatus};
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Al arrancar, borra las carpetas de worktree que no pertenecen a ninguna tarea abierta:
/// las de tareas ya cerradas (`Merged`/`Discarded`) o sin tarea en la base de datos.
pub struct SweepOrphanWorktrees {
    repository: Arc<dyn TaskRepositoryPort>,
    worktrees: Arc<dyn WorktreePort>,
}

impl SweepOrphanWorktrees {
    pub fn new(repository: Arc<dyn TaskRepositoryPort>, worktrees: Arc<dyn WorktreePort>) -> Self {
        Self {
            repository,
            worktrees,
        }
    }

    /// Devuelve cuántas carpetas huérfanas se han borrado.
    pub async fn execute(&self) -> Result<usize, TaskError> {
        let open: Vec<TaskStatus> = TaskStatus::ALL
            .into_iter()
            .filter(|status: &TaskStatus| !status.is_closed())
            .collect();
        let keep: Vec<PathBuf> = self
            .repository
            .list_by_status(&open)
            .await?
            .into_iter()
            .map(|task: Task| task.worktree_path)
            .collect();
        self.worktrees.sweep_orphans(&keep).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::{MockTaskRepository, MockWorktree};

    #[tokio::test]
    async fn keeps_only_the_paths_of_open_tasks() {
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository = test_support::repository_with(None, &recorded);
        repository
            .expect_list_by_status()
            .withf(|statuses: &[TaskStatus]| {
                !statuses.contains(&TaskStatus::Merged)
                    && !statuses.contains(&TaskStatus::Discarded)
                    && statuses.contains(&TaskStatus::Backlog)
                    && statuses.contains(&TaskStatus::Interrupted)
            })
            .returning(|_| Ok(vec![test_support::task(TaskStatus::Backlog)]));
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_sweep_orphans()
            .withf(|keep: &[PathBuf]| keep == [PathBuf::from(test_support::WORKTREE)])
            .returning(|_| Ok(3));

        let removed: usize = SweepOrphanWorktrees::new(Arc::new(repository), Arc::new(worktrees))
            .execute()
            .await
            .expect("ok");

        assert_eq!(removed, 3);
    }

    #[tokio::test]
    async fn propagates_repository_errors_without_sweeping() {
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository = test_support::repository_with(None, &recorded);
        repository
            .expect_list_by_status()
            .returning(|_| Err(TaskError::Repository("boom".to_string())));
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_sweep_orphans().never();

        let result: Result<usize, TaskError> =
            SweepOrphanWorktrees::new(Arc::new(repository), Arc::new(worktrees))
                .execute()
                .await;

        assert!(result.is_err());
    }
}
