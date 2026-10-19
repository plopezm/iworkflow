use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::code_hosting::application::services::task_project_resolver::TaskProjectResolver;
use crate::modules::code_hosting::application::usecases::create_task_merge_request::CreateTaskMergeRequest;
use crate::modules::code_hosting::application::usecases::get_code_host_settings::GetCodeHostSettings;
use crate::modules::code_hosting::application::usecases::get_task_merge_request::GetTaskMergeRequest;
use crate::modules::code_hosting::application::usecases::merge_task_merge_request::MergeTaskMergeRequest;
use crate::modules::code_hosting::application::usecases::save_code_host_settings::SaveCodeHostSettings;
use crate::modules::code_hosting::application::usecases::test_code_host_connection::TestCodeHostConnection;
use crate::modules::code_hosting::domain::ports::outbound::git_remote_port::GitRemotePort;
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_factory_port::CodeHostClientFactoryPort;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;
use crate::modules::code_hosting::domain::ports::outbound::task_source_port::TaskSourcePort;
use crate::modules::code_hosting::infrastructure::adapters::outbound::git::git_cli_remote_adapter::GitCliRemoteAdapter;
use crate::modules::code_hosting::infrastructure::adapters::outbound::http::http_client_manager::HttpClientManager;
use crate::modules::code_hosting::infrastructure::adapters::outbound::persistence::code_host_settings_repository_adapter::CodeHostSettingsRepositoryAdapter;
use crate::modules::code_hosting::infrastructure::factories::code_host_client_factory::CodeHostClientFactory;

#[derive(Clone)]
pub struct CodeHostContext {
    pub get_settings: Arc<GetCodeHostSettings>,
    pub save_settings: Arc<SaveCodeHostSettings>,
    pub test_connection: Arc<TestCodeHostConnection>,
    pub get_task_merge_request: Arc<GetTaskMergeRequest>,
    pub create_task_merge_request: Arc<CreateTaskMergeRequest>,
    pub merge_task_merge_request: Arc<MergeTaskMergeRequest>,
}

impl CodeHostContext {
    /// `tasks` lo aporta el composition root (el feature no conoce el módulo de tareas).
    pub fn new(pool: DbPool, tasks: Arc<dyn TaskSourcePort>) -> Self {
        let repository: Arc<dyn CodeHostSettingsRepositoryPort> =
            Arc::new(CodeHostSettingsRepositoryAdapter::new(pool));
        let clients: Arc<dyn CodeHostClientFactoryPort> = Arc::new(
            CodeHostClientFactory::with_clients(Arc::new(HttpClientManager::new())),
        );
        let remote: Arc<dyn GitRemotePort> = Arc::new(GitCliRemoteAdapter::new());
        let resolver: Arc<TaskProjectResolver> = Arc::new(TaskProjectResolver::new(
            repository.clone(),
            tasks,
            remote.clone(),
            clients.clone(),
        ));
        Self {
            get_settings: Arc::new(GetCodeHostSettings::new(repository.clone())),
            save_settings: Arc::new(SaveCodeHostSettings::new(repository.clone())),
            test_connection: Arc::new(TestCodeHostConnection::new(repository, clients)),
            get_task_merge_request: Arc::new(GetTaskMergeRequest::new(resolver.clone())),
            create_task_merge_request: Arc::new(CreateTaskMergeRequest::new(
                resolver.clone(),
                remote.clone(),
            )),
            merge_task_merge_request: Arc::new(MergeTaskMergeRequest::new(resolver, remote)),
        }
    }
}
