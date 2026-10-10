use std::sync::Arc;

use crate::modules::project_tree::application::usecases::list_project_directory::ListProjectDirectory;
use crate::modules::project_tree::domain::ports::outbound::directory_reader_port::DirectoryReaderPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;
use crate::modules::project_tree::infrastructure::adapters::outbound::fs::fs_directory_reader_adapter::FsDirectoryReaderAdapter;
use crate::modules::project_tree::infrastructure::adapters::outbound::workspace_root::resolver_workspace_root_adapter::{
    ResolverWorkspaceRootAdapter, RootResolver,
};

#[derive(Clone)]
pub struct ProjectTreeContext {
    pub list_directory: Arc<ListProjectDirectory>,
}

impl ProjectTreeContext {
    pub fn new(resolver: RootResolver) -> Self {
        let roots: Arc<dyn WorkspaceRootPort> =
            Arc::new(ResolverWorkspaceRootAdapter::new(resolver));
        let reader: Arc<dyn DirectoryReaderPort> = Arc::new(FsDirectoryReaderAdapter::new());
        Self {
            list_directory: Arc::new(ListProjectDirectory::new(roots, reader)),
        }
    }
}
