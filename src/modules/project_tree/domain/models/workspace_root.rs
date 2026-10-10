use std::path::PathBuf;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkspaceRoot {
    pub name: String,
    pub path: PathBuf,
}
