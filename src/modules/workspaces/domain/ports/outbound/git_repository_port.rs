use std::path::{Path, PathBuf};

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;

/// Información básica del repositorio git de una carpeta.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GitRepositoryInfo {
    pub root: PathBuf,
    /// Rama simbólica de HEAD, incluso si aún no tiene commits (unborn). `None` solo si HEAD está desacoplado.
    pub current_branch: Option<String>,
    /// `true` si el repositorio tiene al menos un commit alcanzable desde HEAD.
    pub has_commits: bool,
}

#[async_trait::async_trait]
pub trait GitRepositoryPort: Send + Sync {
    /// Falla con `PathNotFound` o `NotAGitRepository` según corresponda.
    async fn inspect(&self, path: &Path) -> Result<GitRepositoryInfo, WorkspaceError>;
    async fn branch_exists(&self, root: &Path, branch: &str) -> Result<bool, WorkspaceError>;
    /// Crea la carpeta si falta y ejecuta `git init` en la rama indicada.
    async fn init_repository(
        &self,
        path: &Path,
        initial_branch: &str,
    ) -> Result<(), WorkspaceError>;
    /// Crea un commit inicial vacío (sin tocar el índice) para que HEAD deje de ser unborn.
    async fn create_initial_commit(&self, root: &Path) -> Result<(), WorkspaceError>;
}
