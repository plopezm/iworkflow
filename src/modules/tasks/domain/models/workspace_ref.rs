use std::path::PathBuf;

/// Vista del workspace que necesita el feature de tareas.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkspaceRef {
    pub id: String,
    pub name: String,
    pub path: PathBuf,
    pub base_branch: String,
}
