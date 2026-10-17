use std::path::{Path, PathBuf};
use std::sync::Arc;

use crate::modules::code_hosting::application::services::task_project_resolver::TaskProjectResolver;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::models::merge_request::MergeRequest;
use crate::modules::code_hosting::domain::ports::outbound::task_source_port::TaskSource;
use crate::modules::code_hosting::mocks::{
    MockCodeHostClient, MockCodeHostSettingsRepository, MockGitRemote, MockTaskSourceLookup,
};

pub const TASK_ID: &str = "task-1";
pub const PROJECT: &str = "grupo/proyecto";

pub fn settings() -> CodeHostSettings {
    CodeHostSettings::new(HostingProvider::GitLab, "https://git.corp", "tok", false)
        .expect("settings")
}

pub fn source() -> TaskSource {
    TaskSource {
        repo_path: PathBuf::from("/repo"),
        branch: "pi/feature".to_string(),
        base_branch: "main".to_string(),
        title: "Mi tarea".to_string(),
        description: "Prompt de la tarea".to_string(),
    }
}

pub fn merge_request(iid: u64, state: &str) -> MergeRequest {
    MergeRequest {
        iid,
        title: "Mi tarea".to_string(),
        state: state.to_string(),
        source_branch: "pi/feature".to_string(),
        target_branch: "main".to_string(),
        web_url: format!("https://git.corp/{PROJECT}/-/merge_requests/{iid}"),
        merge_status: "mergeable".to_string(),
        has_conflicts: false,
    }
}

/// Resolver con ajustes, tarea y `origin` válidos sobre el cliente indicado.
pub fn resolver(client: MockCodeHostClient) -> Arc<TaskProjectResolver> {
    let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
    repository.expect_list().returning(|| Ok(vec![settings()]));
    let mut tasks: MockTaskSourceLookup = MockTaskSourceLookup::new();
    tasks.expect_find().returning(|_| Ok(Some(source())));
    let mut remote: MockGitRemote = MockGitRemote::new();
    remote
        .expect_origin_url()
        .withf(|repo: &Path| repo == Path::new("/repo"))
        .returning(|_| Ok(format!("git@git.corp:{PROJECT}.git")));
    Arc::new(TaskProjectResolver::new(
        Arc::new(repository),
        Arc::new(tasks),
        Arc::new(remote),
        Arc::new(client),
    ))
}
