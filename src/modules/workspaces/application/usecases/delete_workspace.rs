use std::sync::Arc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::WorkspaceId;
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use crate::modules::workspaces::domain::ports::outbound::workspace_tasks_port::WorkspaceTasksPort;

/// Elimina el registro de un workspace (no toca la carpeta en disco), borrando antes
/// sus tareas inactivas en cascada.
pub struct DeleteWorkspace {
    repository: Arc<dyn WorkspaceRepositoryPort>,
    tasks: Arc<dyn WorkspaceTasksPort>,
}

impl DeleteWorkspace {
    pub fn new(
        repository: Arc<dyn WorkspaceRepositoryPort>,
        tasks: Arc<dyn WorkspaceTasksPort>,
    ) -> Self {
        Self { repository, tasks }
    }

    pub async fn execute(&self, id: WorkspaceId) -> Result<(), WorkspaceError> {
        self.tasks.delete_all(id).await?;
        self.repository.delete(id).await
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::workspaces::mocks::{MockWorkspaceRepository, MockWorkspaceTasks};

    #[tokio::test]
    async fn deletes_tasks_before_deleting_the_workspace() {
        let id: WorkspaceId = WorkspaceId::new();
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository
            .expect_delete()
            .with(eq(id))
            .times(1)
            .returning(|_| Ok(()));
        let mut tasks: MockWorkspaceTasks = MockWorkspaceTasks::new();
        tasks
            .expect_delete_all()
            .with(eq(id))
            .times(1)
            .returning(|_| Ok(()));
        let use_case: DeleteWorkspace = DeleteWorkspace::new(Arc::new(repository), Arc::new(tasks));

        assert_eq!(use_case.execute(id).await, Ok(()));
    }

    #[tokio::test]
    async fn does_not_delete_workspace_when_tasks_deletion_fails() {
        let id: WorkspaceId = WorkspaceId::new();
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository.expect_delete().times(0);
        let mut tasks: MockWorkspaceTasks = MockWorkspaceTasks::new();
        tasks
            .expect_delete_all()
            .with(eq(id))
            .times(1)
            .returning(|_| Err(WorkspaceError::InUse));
        let use_case: DeleteWorkspace = DeleteWorkspace::new(Arc::new(repository), Arc::new(tasks));

        assert_eq!(use_case.execute(id).await, Err(WorkspaceError::InUse));
    }

    #[tokio::test]
    async fn delegates_to_repository() {
        let id: WorkspaceId = WorkspaceId::new();
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository
            .expect_delete()
            .with(eq(id))
            .times(1)
            .returning(|_| Err(WorkspaceError::InUse));
        let mut tasks: MockWorkspaceTasks = MockWorkspaceTasks::new();
        tasks.expect_delete_all().returning(|_| Ok(()));
        let use_case: DeleteWorkspace = DeleteWorkspace::new(Arc::new(repository), Arc::new(tasks));

        assert_eq!(use_case.execute(id).await, Err(WorkspaceError::InUse));
    }
}
