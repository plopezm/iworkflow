use std::path::{Path, PathBuf};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::worktree::{MergeOutcome, TaskDiff};

/// Operaciones git sobre los worktrees de las tareas.
#[async_trait::async_trait]
pub trait WorktreePort: Send + Sync {
    /// Carpeta donde crear el worktree de una tarea de ese workspace.
    fn worktree_path(&self, workspace_name: &str, dir_name: &str) -> PathBuf;

    /// Crea `branch` desde `base_branch` y la saca en `path`.
    async fn create(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        path: &Path,
    ) -> Result<(), TaskError>;

    /// Hace commit de todos los cambios pendientes. Devuelve `false` si no había nada.
    async fn commit_all(&self, path: &Path, message: &str) -> Result<bool, TaskError>;

    async fn diff(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
    ) -> Result<TaskDiff, TaskError>;

    /// Integra `branch` en `base_branch` con `--no-ff`. Ante conflicto deja todo como estaba.
    async fn merge(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        message: &str,
    ) -> Result<MergeOutcome, TaskError>;

    /// Elimina el worktree (si existe) y la rama.
    async fn remove(&self, repo: &Path, path: &Path, branch: &str) -> Result<(), TaskError>;
}
