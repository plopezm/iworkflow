use std::path::Path;

use crate::modules::project_tree::domain::models::project_scope::ProjectScope;
use mockall::mock;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::directory_contents::DirectoryContents;
use crate::modules::project_tree::domain::models::git_change::GitChange;
use crate::modules::project_tree::domain::models::project_file::ProjectFile;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::directory_reader_port::DirectoryReaderPort;
use crate::modules::project_tree::domain::ports::outbound::file_search_port::FileSearchPort;
use crate::modules::project_tree::domain::ports::outbound::file_store_port::FileStorePort;
use crate::modules::project_tree::domain::ports::outbound::git_status_port::GitStatusPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

mock! {
    pub WorkspaceRoot {}

    #[async_trait::async_trait]
    impl WorkspaceRootPort for WorkspaceRoot {
        async fn resolve(&self, scope: ProjectScope) -> Result<WorkspaceRoot, ProjectTreeError>;
    }
}

mock! {
    pub DirectoryReader {}

    #[async_trait::async_trait]
    impl DirectoryReaderPort for DirectoryReader {
        async fn list(&self, root: &Path, relative: &str) -> Result<DirectoryContents, ProjectTreeError>;
    }
}

mock! {
    pub FileStore {}

    #[async_trait::async_trait]
    impl FileStorePort for FileStore {
        async fn read(&self, root: &Path, relative: &str) -> Result<ProjectFile, ProjectTreeError>;
        async fn write(&self, root: &Path, relative: &str, content: &str) -> Result<(), ProjectTreeError>;
    }
}

mock! {
    pub GitStatus {}

    #[async_trait::async_trait]
    impl GitStatusPort for GitStatus {
        async fn changes(&self, root: &Path) -> Result<Vec<GitChange>, ProjectTreeError>;
        async fn base_content(&self, root: &Path, relative: &str) -> Result<Option<String>, ProjectTreeError>;
        async fn stage(&self, root: &Path, paths: &[String]) -> Result<(), ProjectTreeError>;
        async fn unstage(&self, root: &Path, paths: &[String]) -> Result<(), ProjectTreeError>;
    }
}

mock! {
    pub FileSearch {}

    #[async_trait::async_trait]
    impl FileSearchPort for FileSearch {
        async fn files(&self, root: &Path) -> Result<Vec<String>, ProjectTreeError>;
    }
}
