use std::sync::Arc;

use crate::modules::project_tree::application::usecases::list_project_directory::ListProjectDirectory;
use crate::modules::project_tree::application::usecases::read_project_file::ReadProjectFile;
use crate::modules::project_tree::application::usecases::write_project_file::WriteProjectFile;
use crate::modules::project_tree::domain::ports::outbound::file_store_port::FileStorePort;
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
}

impl ProjectTreeContext {
    pub fn new(resolver: RootResolver) -> Self {
        let roots: Arc<dyn WorkspaceRootPort> =
            Arc::new(ResolverWorkspaceRootAdapter::new(resolver));
        let reader: Arc<dyn DirectoryReaderPort> = Arc::new(FsDirectoryReaderAdapter::new());
        let files: Arc<dyn FileStorePort> = Arc::new(FsFileStoreAdapter::new());
        Self {
            list_directory: Arc::new(ListProjectDirectory::new(roots.clone(), reader)),
            read_file: Arc::new(ReadProjectFile::new(roots.clone(), files.clone())),
            write_file: Arc::new(WriteProjectFile::new(roots, files)),
        }
    }
}
