use std::path::PathBuf;

/// Workspace resuelto para abrir un terminal: nombre (snapshot) y directorio de trabajo.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkspaceTarget {
    pub name: String,
    pub path: PathBuf,
}
