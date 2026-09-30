use std::path::PathBuf;

/// Proyecto (carpeta) al que se puede añadir un AGENTS.md.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProjectRef {
    pub id: String,
    pub name: String,
    pub path: PathBuf,
}
