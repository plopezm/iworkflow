use std::path::Path;

use mockall::mock;
use uuid::Uuid;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::directory_contents::DirectoryContents;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::directory_reader_port::DirectoryReaderPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

mock! {
    pub WorkspaceRoot {}

    #[async_trait::async_trait]
    impl WorkspaceRootPort for WorkspaceRoot {
        async fn resolve(&self, workspace_id: Uuid) -> Result<WorkspaceRoot, ProjectTreeError>;
    }
}

mock! {
    pub DirectoryReader {}

    #[async_trait::async_trait]
    impl DirectoryReaderPort for DirectoryReader {
        async fn list(&self, root: &Path, relative: &str) -> Result<DirectoryContents, ProjectTreeError>;
    }
}
