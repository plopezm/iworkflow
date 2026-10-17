use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::merge_request::{
    MergeRequest, NewMergeRequest, Pipeline,
};

/// Cliente de la API REST v4 de GitLab. `project` es `namespace/proyecto`.
#[async_trait::async_trait]
pub trait CodeHostClientPort: Send + Sync {
    /// Usuario autenticado por el token (sirve para probar la conexión).
    async fn current_user(&self, settings: &CodeHostSettings) -> Result<String, CodeHostError>;
    async fn branch_exists(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        branch: &str,
    ) -> Result<bool, CodeHostError>;
    /// MR más reciente (de cualquier estado) cuya rama origen es `source_branch`.
    async fn find_latest_merge_request(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        source_branch: &str,
    ) -> Result<Option<MergeRequest>, CodeHostError>;
    async fn create_merge_request(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        new: &NewMergeRequest,
    ) -> Result<MergeRequest, CodeHostError>;
    async fn get_merge_request(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        iid: u64,
    ) -> Result<MergeRequest, CodeHostError>;
    /// Último pipeline del MR con sus jobs; `None` si no tiene.
    async fn latest_pipeline(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        iid: u64,
    ) -> Result<Option<Pipeline>, CodeHostError>;
    async fn merge_merge_request(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        iid: u64,
    ) -> Result<MergeRequest, CodeHostError>;
}
