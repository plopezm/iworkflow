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

/// Tarea con la conexión GitLab y el proyecto deducido de su `origin`.
#[derive(Debug, Clone)]
pub struct ResolvedTask {
    pub settings: CodeHostSettings,
    pub source: TaskSource,
    /// `namespace/proyecto` dentro de la instancia configurada.
    pub project: String,
}

/// Une ajustes + tarea + `origin` para localizar el proyecto GitLab de una tarea.
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
        let settings: CodeHostSettings = self
            .settings
            .get()
            .await?
            .ok_or(CodeHostError::NotConfigured)?;
        let source: TaskSource = self
            .tasks
            .find(task_id)
            .await?
            .ok_or_else(|| CodeHostError::TaskNotFound(task_id.to_string()))?;
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
