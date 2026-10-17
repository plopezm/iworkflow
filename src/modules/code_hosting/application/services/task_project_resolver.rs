use std::sync::Arc;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::merge_request::{
    MergeRequest, MergeRequestStatus, Pipeline,
};
use crate::modules::code_hosting::domain::models::remote_url::RemoteUrl;
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_port::CodeHostClientPort;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;
use crate::modules::code_hosting::domain::ports::outbound::git_remote_port::GitRemotePort;
use crate::modules::code_hosting::domain::ports::outbound::task_source_port::{
    TaskSource, TaskSourcePort,
};

/// Tarea con la conexión del proveedor detectado y el proyecto deducido de su `origin`.
#[derive(Debug, Clone)]
pub struct ResolvedTask {
    pub settings: CodeHostSettings,
    pub source: TaskSource,
    /// `namespace/proyecto` (GitLab) u `propietario/repositorio` (GitHub).
    pub project: String,
}

/// Une ajustes + tarea + `origin` para localizar el proveedor y el proyecto de una tarea.
pub struct TaskProjectResolver {
    settings: Arc<dyn CodeHostSettingsRepositoryPort>,
    tasks: Arc<dyn TaskSourcePort>,
    remote: Arc<dyn GitRemotePort>,
    client: Arc<dyn CodeHostClientPort>,
}

impl TaskProjectResolver {
    pub fn new(
        settings: Arc<dyn CodeHostSettingsRepositoryPort>,
        tasks: Arc<dyn TaskSourcePort>,
        remote: Arc<dyn GitRemotePort>,
        client: Arc<dyn CodeHostClientPort>,
    ) -> Self {
        Self {
            settings,
            tasks,
            remote,
            client,
        }
    }

    pub async fn resolve(&self, task_id: &str) -> Result<ResolvedTask, CodeHostError> {
        let source: TaskSource = self
            .tasks
            .find(task_id)
            .await?
            .ok_or_else(|| CodeHostError::TaskNotFound(task_id.to_string()))?;
        let origin: String = self.remote.origin_url(&source.repo_path).await?;
        let remote: RemoteUrl = RemoteUrl::parse(&origin)?;
        // El proveedor lo decide el host del origin: se aplican los ajustes que le corresponden.
        let configured: Vec<CodeHostSettings> = self.settings.list().await?;
        let settings: CodeHostSettings =
            CodeHostSettings::select_for_host(&configured, &remote.host)?.clone();
        let project: String = settings.project_path(&remote)?;
        Ok(ResolvedTask {
            settings,
            source,
            project,
        })
    }

    /// Completa un MR con su último pipeline y jobs.
    pub async fn status_of(
        &self,
        resolved: &ResolvedTask,
        merge_request: MergeRequest,
    ) -> Result<MergeRequestStatus, CodeHostError> {
        let pipeline: Option<Pipeline> = self
            .client
            .latest_pipeline(&resolved.settings, &resolved.project, merge_request.iid)
            .await?;
        Ok(MergeRequestStatus {
            project_path: resolved.project.clone(),
            merge_request,
            pipeline,
        })
    }

    pub fn client(&self) -> &Arc<dyn CodeHostClientPort> {
        &self.client
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::code_hosting::application::test_support::{self, PROJECT, TASK_ID};
    use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
    use crate::modules::code_hosting::mocks::{
        MockCodeHostClient, MockCodeHostSettingsRepository, MockGitRemote, MockTaskSourceLookup,
    };

    fn resolver_with(
        origin: &'static str,
        configured: Vec<CodeHostSettings>,
    ) -> TaskProjectResolver {
        let mut repository: MockCodeHostSettingsRepository = MockCodeHostSettingsRepository::new();
        repository
            .expect_list()
            .returning(move || Ok(configured.clone()));
        let mut tasks: MockTaskSourceLookup = MockTaskSourceLookup::new();
        tasks
            .expect_find()
            .returning(|_| Ok(Some(test_support::source())));
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote
            .expect_origin_url()
            .returning(move |_| Ok(origin.to_string()));
        TaskProjectResolver::new(
            Arc::new(repository),
            Arc::new(tasks),
            Arc::new(remote),
            Arc::new(MockCodeHostClient::new()),
        )
    }

    fn github() -> CodeHostSettings {
        CodeHostSettings::new(HostingProvider::GitHub, "https://github.com", "ghp", false)
            .expect("settings")
    }

    #[tokio::test]
    async fn a_gitlab_origin_applies_the_gitlab_settings() {
        let resolver: TaskProjectResolver = resolver_with(
            "git@git.corp:grupo/proyecto.git",
            vec![github(), test_support::settings()],
        );

        let resolved: ResolvedTask = resolver.resolve(TASK_ID).await.expect("resolved");

        assert_eq!(resolved.settings.provider(), HostingProvider::GitLab);
        assert_eq!(resolved.project, PROJECT);
    }

    #[tokio::test]
    async fn a_github_origin_applies_the_github_settings() {
        let resolver: TaskProjectResolver = resolver_with(
            "git@github.com:acme/app.git",
            vec![test_support::settings(), github()],
        );

        let resolved: ResolvedTask = resolver.resolve(TASK_ID).await.expect("resolved");

        assert_eq!(resolved.settings.provider(), HostingProvider::GitHub);
        assert_eq!(resolved.project, "acme/app");
    }

    #[tokio::test]
    async fn a_github_origin_without_github_settings_asks_to_configure_it() {
        let resolver: TaskProjectResolver = resolver_with(
            "https://github.com/acme/app.git",
            vec![test_support::settings()],
        );

        let result: Result<ResolvedTask, CodeHostError> = resolver.resolve(TASK_ID).await;

        assert_eq!(
            result.unwrap_err(),
            CodeHostError::NotConfigured(HostingProvider::GitHub)
        );
    }

    #[tokio::test]
    async fn an_origin_matching_no_provider_is_reported_with_its_host() {
        let resolver: TaskProjectResolver = resolver_with(
            "https://otro.io/a/b.git",
            vec![test_support::settings(), github()],
        );

        let result: Result<ResolvedTask, CodeHostError> = resolver.resolve(TASK_ID).await;

        assert_eq!(
            result.unwrap_err(),
            CodeHostError::UnknownHost("otro.io".to_string())
        );
    }
}
