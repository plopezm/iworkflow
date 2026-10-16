use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use crate::modules::gitlab::domain::models::merge_request::{
    MergeRequest, NewMergeRequest, Pipeline,
};

/// Cliente de la API REST v4 de GitLab. `project` es `namespace/proyecto`.
#[async_trait::async_trait]
pub trait GitLabClientPort: Send + Sync {
    /// Usuario autenticado por el token (sirve para probar la conexión).
    async fn current_user(&self, settings: &GitLabSettings) -> Result<String, GitLabError>;
    async fn branch_exists(
        &self,
        settings: &GitLabSettings,
        project: &str,
        branch: &str,
    ) -> Result<bool, GitLabError>;
    /// MR más reciente (de cualquier estado) cuya rama origen es `source_branch`.
    async fn find_latest_merge_request(
        &self,
        settings: &GitLabSettings,
        project: &str,
        source_branch: &str,
    ) -> Result<Option<MergeRequest>, GitLabError>;
    async fn create_merge_request(
        &self,
        settings: &GitLabSettings,
        project: &str,
        new: &NewMergeRequest,
    ) -> Result<MergeRequest, GitLabError>;
    async fn get_merge_request(
        &self,
        settings: &GitLabSettings,
        project: &str,
        iid: u64,
    ) -> Result<MergeRequest, GitLabError>;
    /// Último pipeline del MR con sus jobs; `None` si no tiene.
    async fn latest_pipeline(
        &self,
        settings: &GitLabSettings,
        project: &str,
        iid: u64,
    ) -> Result<Option<Pipeline>, GitLabError>;
    async fn merge_merge_request(
        &self,
        settings: &GitLabSettings,
        project: &str,
        iid: u64,
    ) -> Result<MergeRequest, GitLabError>;
}
