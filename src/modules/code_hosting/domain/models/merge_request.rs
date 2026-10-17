use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;

/// Datos para abrir un merge request.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NewMergeRequest {
    pub source_branch: String,
    pub target_branch: String,
    pub title: String,
    pub description: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MergeRequest {
    pub iid: u64,
    pub title: String,
    /// `opened`, `merged`, `closed`, `locked`.
    pub state: String,
    pub source_branch: String,
    pub target_branch: String,
    pub web_url: String,
    /// `can_be_merged`, `cannot_be_merged`, `checking`, etc.
    pub merge_status: String,
    pub has_conflicts: bool,
}

impl MergeRequest {
    pub fn is_open(&self) -> bool {
        self.state == "opened"
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PipelineJob {
    pub id: u64,
    pub name: String,
    pub stage: String,
    pub status: String,
    pub web_url: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Pipeline {
    pub id: u64,
    pub status: String,
    pub web_url: String,
    pub jobs: Vec<PipelineJob>,
}

/// Estado agregado de un MR/PR: el propio MR y el último pipeline con sus jobs.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MergeRequestStatus {
    /// Proveedor que aloja el repositorio (decide el vocabulario de la UI).
    pub provider: HostingProvider,
    pub project_path: String,
    pub merge_request: MergeRequest,
    pub pipeline: Option<Pipeline>,
}
