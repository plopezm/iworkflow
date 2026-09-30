use std::path::Path;

use mockall::mock;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::agents_file_option::AgentsFileOption;
use crate::modules::workspaces::domain::models::directory_listing::DirectoryListing;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use crate::modules::workspaces::domain::ports::outbound::agents_files_port::AgentsFilesPort;
use crate::modules::workspaces::domain::ports::outbound::directory_browser_port::DirectoryBrowserPort;
use crate::modules::workspaces::domain::ports::outbound::git_repository_port::{
    GitRepositoryInfo, GitRepositoryPort,
};
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use crate::modules::workspaces::domain::ports::outbound::workspace_tasks_port::WorkspaceTasksPort;

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
    pub WorkspaceTasks {}

    #[async_trait::async_trait]
    impl WorkspaceTasksPort for WorkspaceTasks {
        async fn delete_all(&self, id: WorkspaceId) -> Result<(), WorkspaceError>;
    }
}

mock! {
    pub AgentsFiles {}

    #[async_trait::async_trait]
    impl AgentsFilesPort for AgentsFiles {
        async fn list_options(&self) -> Result<Vec<AgentsFileOption>, WorkspaceError>;
        async fn install(&self, agents_file_id: &str, root: &Path) -> Result<(), WorkspaceError>;
    }
}

mock! {
    pub DirectoryBrowser {}

    #[async_trait::async_trait]
    impl DirectoryBrowserPort for DirectoryBrowser {
        async fn list(&self, path: &Path) -> Result<DirectoryListing, WorkspaceError>;
    }
}

mock! {
    pub GitRepository {}

    #[async_trait::async_trait]
    impl GitRepositoryPort for GitRepository {
        async fn inspect(&self, path: &Path) -> Result<GitRepositoryInfo, WorkspaceError>;
        async fn branch_exists(&self, root: &Path, branch: &str) -> Result<bool, WorkspaceError>;
        async fn init_repository(&self, path: &Path, initial_branch: &str) -> Result<(), WorkspaceError>;
        async fn create_initial_commit(&self, root: &Path) -> Result<(), WorkspaceError>;
    }
}
