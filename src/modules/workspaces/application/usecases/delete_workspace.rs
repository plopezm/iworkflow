use std::sync::Arc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::WorkspaceId;
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;

/// Elimina el registro de un workspace (no toca la carpeta en disco).
pub struct DeleteWorkspace {
    repository: Arc<dyn WorkspaceRepositoryPort>,
}

impl DeleteWorkspace {
    pub fn new(repository: Arc<dyn WorkspaceRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self, id: WorkspaceId) -> Result<(), WorkspaceError> {
        self.repository.delete(id).await
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::workspaces::mocks::MockWorkspaceRepository;

    #[tokio::test]
    async fn delegates_to_repository() {
        let id: WorkspaceId = WorkspaceId::new();
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository
            .expect_delete()
            .with(eq(id))
            .times(1)
            .returning(|_| Err(WorkspaceError::InUse));
        let use_case: DeleteWorkspace = DeleteWorkspace::new(Arc::new(repository));

        assert_eq!(use_case.execute(id).await, Err(WorkspaceError::InUse));
    }
}
