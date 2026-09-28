use std::path::{Path, PathBuf};

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;

/// Información básica del repositorio git de una carpeta.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GitRepositoryInfo {
    pub root: PathBuf,
    /// `None` si HEAD está desacoplado o el repositorio no tiene commits.
    pub current_branch: Option<String>,
}

#[async_trait::async_trait]
pub trait GitRepositoryPort: Send + Sync {
    /// Falla con `PathNotFound` o `NotAGitRepository` según corresponda.
    async fn inspect(&self, path: &Path) -> Result<GitRepositoryInfo, WorkspaceError>;
    async fn branch_exists(&self, root: &Path, branch: &str) -> Result<bool, WorkspaceError>;
}
