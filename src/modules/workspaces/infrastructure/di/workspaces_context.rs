use std::path::PathBuf;
use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::workspaces::application::usecases::browse_directories::BrowseDirectories;
use crate::modules::workspaces::application::usecases::create_directory::CreateDirectory;
use crate::modules::workspaces::application::usecases::delete_workspace::DeleteWorkspace;
use crate::modules::workspaces::application::usecases::get_workspace::GetWorkspace;
use crate::modules::workspaces::application::usecases::list_agents_file_options::ListAgentsFileOptions;
use crate::modules::workspaces::application::usecases::list_workspaces::ListWorkspaces;
use crate::modules::workspaces::application::usecases::register_workspace::RegisterWorkspace;
use crate::modules::workspaces::domain::ports::outbound::agents_files_port::AgentsFilesPort;
use crate::modules::workspaces::domain::ports::outbound::directory_browser_port::DirectoryBrowserPort;
use crate::modules::workspaces::domain::ports::outbound::git_repository_port::GitRepositoryPort;
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use crate::modules::workspaces::domain::ports::outbound::workspace_tasks_port::WorkspaceTasksPort;
use crate::modules::workspaces::infrastructure::adapters::outbound::fs::fs_directory_browser_adapter::FsDirectoryBrowserAdapter;
use crate::modules::workspaces::infrastructure::adapters::outbound::git::git_cli_repository_adapter::GitCliRepositoryAdapter;
use crate::modules::workspaces::infrastructure::adapters::outbound::persistence::workspace_repository_adapter::WorkspaceRepositoryAdapter;

#[derive(Clone)]
pub struct WorkspacesContext {
    pub register_workspace: Arc<RegisterWorkspace>,
    pub list_workspaces: Arc<ListWorkspaces>,
    pub get_workspace: Arc<GetWorkspace>,
    pub delete_workspace: Arc<DeleteWorkspace>,
    pub browse_directories: Arc<BrowseDirectories>,
    pub create_directory: Arc<CreateDirectory>,
    pub list_agents_file_options: Arc<ListAgentsFileOptions>,
}

impl WorkspacesContext {
    pub fn new(
        pool: DbPool,
        home_dir: PathBuf,
        tasks: Arc<dyn WorkspaceTasksPort>,
        agents_files: Arc<dyn AgentsFilesPort>,
    ) -> Self {
        let repository: Arc<dyn WorkspaceRepositoryPort> =
            Arc::new(WorkspaceRepositoryAdapter::new(pool));
        let git: Arc<dyn GitRepositoryPort> = Arc::new(GitCliRepositoryAdapter::new());
        let directory_browser: Arc<dyn DirectoryBrowserPort> =
            Arc::new(FsDirectoryBrowserAdapter::new(home_dir.clone()));
        Self {
            register_workspace: Arc::new(RegisterWorkspace::new(
                repository.clone(),
                git,
                agents_files.clone(),
            )),
            list_workspaces: Arc::new(ListWorkspaces::new(repository.clone())),
            get_workspace: Arc::new(GetWorkspace::new(repository.clone())),
            delete_workspace: Arc::new(DeleteWorkspace::new(repository, tasks)),
            browse_directories: Arc::new(BrowseDirectories::new(
                directory_browser.clone(),
                home_dir,
            )),
            create_directory: Arc::new(CreateDirectory::new(directory_browser)),
            list_agents_file_options: Arc::new(ListAgentsFileOptions::new(agents_files)),
        }
    }
}
