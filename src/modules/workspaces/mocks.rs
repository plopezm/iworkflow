use std::path::Path;

use mockall::mock;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use crate::modules::workspaces::domain::ports::outbound::git_repository_port::{
    GitRepositoryInfo, GitRepositoryPort,
};
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;

mock! {
    pub WorkspaceRepository {}

    #[async_trait::async_trait]
    impl WorkspaceRepositoryPort for WorkspaceRepository {
        async fn save(&self, workspace: &Workspace) -> Result<(), WorkspaceError>;
        async fn list(&self) -> Result<Vec<Workspace>, WorkspaceError>;
        async fn find_by_id(&self, id: WorkspaceId) -> Result<Option<Workspace>, WorkspaceError>;
        async fn find_by_path(&self, path: &Path) -> Result<Option<Workspace>, WorkspaceError>;
        async fn delete(&self, id: WorkspaceId) -> Result<(), WorkspaceError>;
    }
}

mock! {
    pub GitRepository {}

    #[async_trait::async_trait]
    impl GitRepositoryPort for GitRepository {
        async fn inspect(&self, path: &Path) -> Result<GitRepositoryInfo, WorkspaceError>;
        async fn branch_exists(&self, root: &Path, branch: &str) -> Result<bool, WorkspaceError>;
    }
}
