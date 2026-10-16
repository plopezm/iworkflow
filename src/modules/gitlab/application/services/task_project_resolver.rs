use std::sync::Arc;

use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use crate::modules::gitlab::domain::models::merge_request::{
    MergeRequest, MergeRequestStatus, Pipeline,
};
use crate::modules::gitlab::domain::models::remote_url::RemoteUrl;
use crate::modules::gitlab::domain::ports::outbound::git_remote_port::GitRemotePort;
use crate::modules::gitlab::domain::ports::outbound::gitlab_client_port::GitLabClientPort;
use crate::modules::gitlab::domain::ports::outbound::gitlab_settings_repository_port::GitLabSettingsRepositoryPort;
use crate::modules::gitlab::domain::ports::outbound::task_source_port::{
    TaskSource, TaskSourcePort,
};

/// Tarea con la conexión GitLab y el proyecto deducido de su `origin`.
#[derive(Debug, Clone)]
pub struct ResolvedTask {
    pub settings: GitLabSettings,
    pub source: TaskSource,
    /// `namespace/proyecto` dentro de la instancia configurada.
    pub project: String,
}

/// Une ajustes + tarea + `origin` para localizar el proyecto GitLab de una tarea.
pub struct TaskProjectResolver {
    settings: Arc<dyn GitLabSettingsRepositoryPort>,
    tasks: Arc<dyn TaskSourcePort>,
    remote: Arc<dyn GitRemotePort>,
    client: Arc<dyn GitLabClientPort>,
}

impl TaskProjectResolver {
    pub fn new(
        settings: Arc<dyn GitLabSettingsRepositoryPort>,
        tasks: Arc<dyn TaskSourcePort>,
        remote: Arc<dyn GitRemotePort>,
        client: Arc<dyn GitLabClientPort>,
    ) -> Self {
        Self {
            settings,
            tasks,
            remote,
            client,
        }
    }

    pub async fn resolve(&self, task_id: &str) -> Result<ResolvedTask, GitLabError> {
        let settings: GitLabSettings = self
            .settings
            .get()
            .await?
            .ok_or(GitLabError::NotConfigured)?;
        let source: TaskSource = self
            .tasks
            .find(task_id)
            .await?
            .ok_or_else(|| GitLabError::TaskNotFound(task_id.to_string()))?;
        let origin: String = self.remote.origin_url(&source.repo_path).await?;
        let project: String = settings.project_path(&RemoteUrl::parse(&origin)?)?;
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
    ) -> Result<MergeRequestStatus, GitLabError> {
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

    pub fn client(&self) -> &Arc<dyn GitLabClientPort> {
        &self.client
    }
}
