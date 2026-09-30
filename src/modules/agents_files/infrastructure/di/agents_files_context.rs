use std::path::PathBuf;
use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::agents_files::application::usecases::apply_agents_file_to_project::ApplyAgentsFileToProject;
use crate::modules::agents_files::application::usecases::delete_agents_file::DeleteAgentsFile;
use crate::modules::agents_files::application::usecases::install_agents_file::InstallAgentsFile;
use crate::modules::agents_files::application::usecases::list_agents_files::ListAgentsFiles;
use crate::modules::agents_files::application::usecases::list_projects::ListProjects;
use crate::modules::agents_files::application::usecases::upload_agents_file::UploadAgentsFile;
use crate::modules::agents_files::domain::ports::outbound::agents_file_installer_port::AgentsFileInstallerPort;
use crate::modules::agents_files::domain::ports::outbound::agents_file_repository_port::AgentsFileRepositoryPort;
use crate::modules::agents_files::domain::ports::outbound::agents_file_storage_port::AgentsFileStoragePort;
use crate::modules::agents_files::domain::ports::outbound::project_lookup_port::ProjectLookupPort;
use crate::modules::agents_files::infrastructure::adapters::outbound::fs::fs_agents_file_storage_adapter::FsAgentsFileStorageAdapter;
use crate::modules::agents_files::infrastructure::adapters::outbound::git::git_agents_file_installer_adapter::GitAgentsFileInstallerAdapter;
use crate::modules::agents_files::infrastructure::adapters::outbound::persistence::agents_file_repository_adapter::AgentsFileRepositoryAdapter;

#[derive(Clone)]
pub struct AgentsFilesContext {
    pub upload: Arc<UploadAgentsFile>,
    pub list: Arc<ListAgentsFiles>,
    pub delete: Arc<DeleteAgentsFile>,
    pub install: Arc<InstallAgentsFile>,
    pub apply: Arc<ApplyAgentsFileToProject>,
    pub list_projects: Arc<ListProjects>,
}

impl AgentsFilesContext {
    pub fn new(pool: DbPool, storage_dir: PathBuf, projects: Arc<dyn ProjectLookupPort>) -> Self {
        let repository: Arc<dyn AgentsFileRepositoryPort> =
            Arc::new(AgentsFileRepositoryAdapter::new(pool));
        let storage: Arc<dyn AgentsFileStoragePort> =
            Arc::new(FsAgentsFileStorageAdapter::new(storage_dir));
        let installer: Arc<dyn AgentsFileInstallerPort> =
            Arc::new(GitAgentsFileInstallerAdapter::new());
        let install: Arc<InstallAgentsFile> = Arc::new(InstallAgentsFile::new(
            repository.clone(),
            storage.clone(),
            installer,
        ));
        Self {
            upload: Arc::new(UploadAgentsFile::new(repository.clone(), storage.clone())),
            list: Arc::new(ListAgentsFiles::new(repository.clone())),
            delete: Arc::new(DeleteAgentsFile::new(repository, storage)),
            install: install.clone(),
            apply: Arc::new(ApplyAgentsFileToProject::new(projects.clone(), install)),
            list_projects: Arc::new(ListProjects::new(projects)),
        }
    }
}
