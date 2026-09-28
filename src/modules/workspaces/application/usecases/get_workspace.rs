use std::sync::Arc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;

/// Obtiene un workspace por id; `NotFound` si no existe.
pub struct GetWorkspace {
    repository: Arc<dyn WorkspaceRepositoryPort>,
}

impl GetWorkspace {
    pub fn new(repository: Arc<dyn WorkspaceRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self, id: WorkspaceId) -> Result<Workspace, WorkspaceError> {
        self.repository
            .find_by_id(id)
            .await?
            .ok_or_else(|| WorkspaceError::NotFound(id.to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::workspaces::mocks::MockWorkspaceRepository;

    #[tokio::test]
    async fn returns_not_found_when_missing() {
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository.expect_find_by_id().returning(|_| Ok(None));
        let use_case: GetWorkspace = GetWorkspace::new(Arc::new(repository));
        let id: WorkspaceId = WorkspaceId::new();

        let result: Result<Workspace, WorkspaceError> = use_case.execute(id).await;

        assert_eq!(result, Err(WorkspaceError::NotFound(id.to_string())));
    }
}
