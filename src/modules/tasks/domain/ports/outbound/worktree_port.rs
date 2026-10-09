use std::path::{Path, PathBuf};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::worktree::{DiffRange, MergeOutcome, TaskDiff};

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

    /// Fija el rango de commits de `branch` respecto a `base_branch` (merge-base y punta) y crea
    /// la referencia `pin_ref` apuntando a la punta para que sobreviva al borrado de la rama.
    /// Hay que llamarlo ANTES del merge: después, el merge-base coincide con la punta.
    async fn pin_range(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        pin_ref: &str,
    ) -> Result<DiffRange, TaskError>;

    /// Diff de un rango ya fijado; mismo resultado que `diff`, aunque la rama ya no exista.
    async fn diff_range(&self, repo: &Path, range: &DiffRange) -> Result<TaskDiff, TaskError>;

    /// Integra `branch` en `base_branch` con `--no-ff`. Ante conflicto deja todo como estaba.
    async fn merge(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        message: &str,
    ) -> Result<MergeOutcome, TaskError>;

    /// Indica si la carpeta del worktree existe en disco.
    async fn exists(&self, path: &Path) -> bool;

    /// Elimina el worktree (si existe) y la rama.
    async fn remove(&self, repo: &Path, path: &Path, branch: &str) -> Result<(), TaskError>;
}
