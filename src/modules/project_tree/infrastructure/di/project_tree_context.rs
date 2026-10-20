use std::sync::Arc;

use crate::modules::project_tree::application::usecases::list_project_directory::ListProjectDirectory;
use crate::modules::project_tree::application::usecases::list_project_git_changes::ListProjectGitChanges;
use crate::modules::project_tree::application::usecases::read_project_file::ReadProjectFile;
use crate::modules::project_tree::application::usecases::read_project_file_base::ReadProjectFileBase;
use crate::modules::project_tree::application::usecases::search_project_files::SearchProjectFiles;
use crate::modules::project_tree::application::usecases::write_project_file::WriteProjectFile;
use crate::modules::project_tree::domain::ports::outbound::file_search_port::FileSearchPort;
use crate::modules::project_tree::infrastructure::adapters::outbound::git::git_cli_file_search_adapter::GitCliFileSearchAdapter;
use crate::modules::project_tree::domain::ports::outbound::file_store_port::FileStorePort;
use crate::modules::project_tree::domain::ports::outbound::git_status_port::GitStatusPort;
use crate::modules::project_tree::infrastructure::adapters::outbound::git::git_cli_status_adapter::GitCliStatusAdapter;
use crate::modules::project_tree::infrastructure::adapters::outbound::fs::fs_file_store_adapter::FsFileStoreAdapter;
use crate::modules::project_tree::domain::ports::outbound::directory_reader_port::DirectoryReaderPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;
use crate::modules::project_tree::infrastructure::adapters::outbound::fs::fs_directory_reader_adapter::FsDirectoryReaderAdapter;
use crate::modules::project_tree::infrastructure::adapters::outbound::workspace_root::resolver_workspace_root_adapter::{
    ResolverWorkspaceRootAdapter, RootResolver,
};

#[derive(Clone)]
pub struct ProjectTreeContext {
    pub list_directory: Arc<ListProjectDirectory>,
    pub read_file: Arc<ReadProjectFile>,
    pub write_file: Arc<WriteProjectFile>,
    pub git_changes: Arc<ListProjectGitChanges>,
    pub file_base: Arc<ReadProjectFileBase>,
    pub search_files: Arc<SearchProjectFiles>,
}

impl ProjectTreeContext {
    pub fn new(workspaces: RootResolver, tasks: RootResolver) -> Self {
        let roots: Arc<dyn WorkspaceRootPort> =
            Arc::new(ResolverWorkspaceRootAdapter::new(workspaces, tasks));
        let reader: Arc<dyn DirectoryReaderPort> = Arc::new(FsDirectoryReaderAdapter::new());
        let files: Arc<dyn FileStorePort> = Arc::new(FsFileStoreAdapter::new());
        let search: Arc<dyn FileSearchPort> = Arc::new(GitCliFileSearchAdapter::new());
        let git: Arc<dyn GitStatusPort> = Arc::new(GitCliStatusAdapter::new());
        Self {
            list_directory: Arc::new(ListProjectDirectory::new(roots.clone(), reader)),
            read_file: Arc::new(ReadProjectFile::new(roots.clone(), files.clone())),
            write_file: Arc::new(WriteProjectFile::new(roots.clone(), files)),
            git_changes: Arc::new(ListProjectGitChanges::new(roots.clone(), git.clone())),
            file_base: Arc::new(ReadProjectFileBase::new(roots.clone(), git)),
            search_files: Arc::new(SearchProjectFiles::new(roots, search)),
        }
    }
}
