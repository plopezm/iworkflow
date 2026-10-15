use std::path::{Path, PathBuf};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::branch_option::BranchOption;
use crate::modules::tasks::domain::models::worktree::{DiffRange, MergeOutcome, TaskDiff};

/// Operaciones git sobre los worktrees de las tareas.
#[async_trait::async_trait]
pub trait WorktreePort: Send + Sync {
    /// Carpeta donde crear el worktree de una tarea de ese workspace.
    fn worktree_path(&self, workspace_name: &str, dir_name: &str) -> PathBuf;

    /// Ramas locales y remotas del repositorio (sin las de las tareas ni `origin/HEAD`), las
    /// locales primero y cada grupo ordenado por nombre.
    async fn list_branches(&self, repo: &Path) -> Result<Vec<BranchOption>, TaskError>;

    /// Crea `branch` desde `base_branch` (local o remota, p. ej. `origin/develop`) y
    /// la saca en `path`.
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

    /// Huella barata del resultado de `diff`: cambia si y solo si cambia el diff (el diff solo
    /// depende de los commits de ambas ramas, no de cambios sin commitear).
    async fn diff_fingerprint(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
    ) -> Result<String, TaskError>;

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
    /// Si `base_branch` es remota (`origin/develop`) se integra en la rama local homónima
    /// (`develop`), que se crea siguiendo a la remota si aún no existe; nunca se hace push.
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
