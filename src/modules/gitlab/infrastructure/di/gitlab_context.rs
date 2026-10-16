use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::gitlab::application::services::task_project_resolver::TaskProjectResolver;
use crate::modules::gitlab::application::usecases::create_task_merge_request::CreateTaskMergeRequest;
use crate::modules::gitlab::application::usecases::get_gitlab_settings::GetGitLabSettings;
use crate::modules::gitlab::application::usecases::get_task_merge_request::GetTaskMergeRequest;
use crate::modules::gitlab::application::usecases::merge_task_merge_request::MergeTaskMergeRequest;
use crate::modules::gitlab::application::usecases::save_gitlab_settings::SaveGitLabSettings;
use crate::modules::gitlab::application::usecases::test_gitlab_connection::TestGitLabConnection;
use crate::modules::gitlab::domain::ports::outbound::git_remote_port::GitRemotePort;
use crate::modules::gitlab::domain::ports::outbound::gitlab_client_port::GitLabClientPort;
use crate::modules::gitlab::domain::ports::outbound::gitlab_settings_repository_port::GitLabSettingsRepositoryPort;
use crate::modules::gitlab::domain::ports::outbound::task_source_port::TaskSourcePort;
use crate::modules::gitlab::infrastructure::adapters::outbound::git::git_cli_remote_adapter::GitCliRemoteAdapter;
use crate::modules::gitlab::infrastructure::adapters::outbound::http::gitlab_http_client_adapter::GitLabHttpClientAdapter;
use crate::modules::gitlab::infrastructure::adapters::outbound::persistence::gitlab_settings_repository_adapter::GitLabSettingsRepositoryAdapter;

#[derive(Clone)]
pub struct GitLabContext {
    pub get_settings: Arc<GetGitLabSettings>,
    pub save_settings: Arc<SaveGitLabSettings>,
    pub test_connection: Arc<TestGitLabConnection>,
    pub get_task_merge_request: Arc<GetTaskMergeRequest>,
    pub create_task_merge_request: Arc<CreateTaskMergeRequest>,
    pub merge_task_merge_request: Arc<MergeTaskMergeRequest>,
}

impl GitLabContext {
    /// `tasks` lo aporta el composition root (el feature no conoce el módulo de tareas).
    pub fn new(pool: DbPool, tasks: Arc<dyn TaskSourcePort>) -> Self {
        let repository: Arc<dyn GitLabSettingsRepositoryPort> =
            Arc::new(GitLabSettingsRepositoryAdapter::new(pool));
        let client: Arc<dyn GitLabClientPort> = Arc::new(GitLabHttpClientAdapter::new());
        let remote: Arc<dyn GitRemotePort> = Arc::new(GitCliRemoteAdapter::new());
        let resolver: Arc<TaskProjectResolver> = Arc::new(TaskProjectResolver::new(
            repository.clone(),
            tasks,
            remote,
            client.clone(),
        ));
        Self {
            get_settings: Arc::new(GetGitLabSettings::new(repository.clone())),
            save_settings: Arc::new(SaveGitLabSettings::new(repository.clone())),
            test_connection: Arc::new(TestGitLabConnection::new(repository, client)),
            get_task_merge_request: Arc::new(GetTaskMergeRequest::new(resolver.clone())),
            create_task_merge_request: Arc::new(CreateTaskMergeRequest::new(resolver.clone())),
            merge_task_merge_request: Arc::new(MergeTaskMergeRequest::new(resolver)),
        }
    }
}
