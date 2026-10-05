use std::path::{Path, PathBuf};

use crate::core::git::{GitCommandError, GitOutput, run_git, run_git_raw};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::slugify;
use crate::modules::tasks::domain::models::worktree::{MergeOutcome, TaskDiff};
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Tamaño máximo del parche devuelto al cliente.
const MAX_PATCH_BYTES: usize = 1024 * 1024;
/// Identidad usada solo si el repositorio no tiene una configurada.
const FALLBACK_USER_NAME: &str = "iworkflow";
const FALLBACK_USER_EMAIL: &str = "iworkflow@localhost";
/// Subcarpeta (dentro de la raíz de worktrees) para merges temporales.
const MERGE_DIR: &str = ".merge";

/// Worktrees gestionados con la CLI de git bajo una carpeta raíz.
pub struct GitCliWorktreeAdapter {
    root: PathBuf,
    /// Serializa los merges: todos escriben en la misma rama base y compiten por `index.lock`.
    merge_lock: tokio::sync::Mutex<()>,
}

impl GitCliWorktreeAdapter {
    pub fn new(root: PathBuf) -> Self {
        Self {
            root,
            merge_lock: tokio::sync::Mutex::new(()),
        }
    }
}

fn git_error(error: GitCommandError) -> TaskError {
    TaskError::Git(error.to_string())
}

async fn git(dir: &Path, args: &[&str]) -> Result<String, TaskError> {
    run_git(dir, args).await.map_err(git_error)
}

async fn git_raw(dir: &Path, args: &[&str]) -> Result<GitOutput, TaskError> {
    run_git_raw(dir, args).await.map_err(git_error)
}

/// Argumentos `-c user.*` si el repositorio no tiene identidad configurada.
async fn identity_args(dir: &Path) -> Result<Vec<String>, TaskError> {
    let email: GitOutput = git_raw(dir, &["config", "user.email"]).await?;
    let name: GitOutput = git_raw(dir, &["config", "user.name"]).await?;
    let mut args: Vec<String> = Vec::new();
    if !name.success || name.stdout.trim().is_empty() {
        args.extend(["-c".to_string(), format!("user.name={FALLBACK_USER_NAME}")]);
    }
    if !email.success || email.stdout.trim().is_empty() {
        args.extend([
            "-c".to_string(),
            format!("user.email={FALLBACK_USER_EMAIL}"),
        ]);
    }
    Ok(args)
}

/// Ejecuta un comando git anteponiendo la identidad de respaldo si hace falta.
async fn git_with_identity(dir: &Path, args: &[&str]) -> Result<GitOutput, TaskError> {
    let identity: Vec<String> = identity_args(dir).await?;
    let mut full: Vec<&str> = identity.iter().map(String::as_str).collect();
    full.extend_from_slice(args);
    git_raw(dir, &full).await
}

/// Worktree (si lo hay) que tiene `branch` checkouteada, según `git worktree list --porcelain`.
fn worktree_with_branch(porcelain: &str, branch: &str) -> Option<PathBuf> {
    let target: String = format!("branch refs/heads/{branch}");
    let mut current: Option<PathBuf> = None;
    for line in porcelain.lines() {
        if let Some(path) = line.strip_prefix("worktree ") {
            current = Some(PathBuf::from(path));
        } else if line == target {
            return current;
        }
    }
    None
}

impl GitCliWorktreeAdapter {
    /// Ejecuta el merge en `target` (con `base_branch` ya checkouteada).
    async fn merge_in(
        &self,
        target: &Path,
        branch: &str,
        message: &str,
    ) -> Result<MergeOutcome, TaskError> {
        let result: GitOutput = git_with_identity(
            target,
            &["merge", "--no-ff", "--no-edit", "-m", message, branch],
        )
        .await?;
        if result.success {
            return Ok(MergeOutcome::Merged);
        }
        // Sin MERGE_HEAD no hay merge pendiente (p. ej. git lo rechazó antes de empezar).
        let pending: bool = git_raw(target, &["rev-parse", "-q", "--verify", "MERGE_HEAD"])
            .await
            .map(|output| output.success)
            .unwrap_or(false);
        let conflicted: Result<String, TaskError> =
            git(target, &["diff", "--name-only", "--diff-filter=U"]).await;
        if pending {
            // Deja la rama base intacta pase lo que pase.
            let aborted: GitOutput = git_raw(target, &["merge", "--abort"]).await?;
            if !aborted.success {
                return Err(TaskError::Git(format!(
                    "no se pudo abortar el merge en {}; queda un merge pendiente y hay que ejecutar `git merge --abort` manualmente: {}",
                    target.display(),
                    aborted.stderr
                )));
            }
        }
        let files: Vec<String> = conflicted?.lines().map(str::to_string).collect();
        if files.is_empty() {
            return Err(TaskError::Git(format!("merge fallido: {}", result.stderr)));
        }
        Ok(MergeOutcome::Conflict { files })
    }
}

#[async_trait::async_trait]
impl WorktreePort for GitCliWorktreeAdapter {
    fn worktree_path(&self, workspace_name: &str, dir_name: &str) -> PathBuf {
        self.root.join(workspace_name).join(dir_name)
    }

    async fn exists(&self, path: &Path) -> bool {
        matches!(tokio::fs::try_exists(path).await, Ok(true))
    }

    async fn create(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        path: &Path,
    ) -> Result<(), TaskError> {
        if let Some(parent) = path.parent() {
            tokio::fs::create_dir_all(parent).await.map_err(|e| {
                TaskError::Git(format!("no se pudo crear {}: {e}", parent.display()))
            })?;
        }
        let path_arg: String = path.to_string_lossy().into_owned();
        git(
            repo,
            &["worktree", "add", "-b", branch, &path_arg, base_branch],
        )
        .await?;
        Ok(())
    }

    async fn commit_all(&self, path: &Path, message: &str) -> Result<bool, TaskError> {
        git(path, &["add", "-A"]).await?;
        let staged: GitOutput = git_raw(path, &["diff", "--cached", "--quiet"]).await?;
        if staged.success {
            return Ok(false);
        }
        let commit: GitOutput = git_with_identity(path, &["commit", "-q", "-m", message]).await?;
        if !commit.success {
            return Err(TaskError::Git(format!("commit fallido: {}", commit.stderr)));
        }
        Ok(true)
    }

    async fn diff(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
    ) -> Result<TaskDiff, TaskError> {
        let range: String = format!("{base_branch}...{branch}");
        let log_range: String = format!("{base_branch}..{branch}");
        let stat: String = git(repo, &["diff", "--stat", &range]).await?;
        let full_patch: String = git(repo, &["diff", &range]).await?;
        let commits: Vec<String> = git(repo, &["log", "--oneline", &log_range])
            .await?
            .lines()
            .map(str::to_string)
            .collect();
        let truncated: bool = full_patch.len() > MAX_PATCH_BYTES;
        let patch: String = if truncated {
            let mut cut: usize = MAX_PATCH_BYTES;
            while !full_patch.is_char_boundary(cut) {
                cut -= 1;
            }
            full_patch[..cut].to_string()
        } else {
            full_patch
        };
        Ok(TaskDiff {
            stat,
            patch,
            truncated,
            commits,
        })
    }

    async fn merge(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        message: &str,
    ) -> Result<MergeOutcome, TaskError> {
        let _guard: tokio::sync::MutexGuard<'_, ()> = self.merge_lock.lock().await;
        let porcelain: String = git(repo, &["worktree", "list", "--porcelain"]).await?;
        if let Some(checkout) = worktree_with_branch(&porcelain, base_branch) {
            let dirty: String = git(
                &checkout,
                &["status", "--porcelain", "--untracked-files=no"],
            )
            .await?;
            if !dirty.is_empty() {
                return Err(TaskError::MergeBlocked(format!(
                    "la rama {base_branch} tiene cambios sin commitear en {}",
                    checkout.display()
                )));
            }
            return self.merge_in(&checkout, branch, message).await;
        }

        // La rama base no está checkouteada: se integra en un worktree temporal.
        let unique: String = format!("{}-{}", slugify(branch), uuid::Uuid::new_v4());
        let temp: PathBuf = self.root.join(MERGE_DIR).join(unique);
        if let Some(parent) = temp.parent() {
            tokio::fs::create_dir_all(parent).await.map_err(|e| {
                TaskError::Git(format!("no se pudo crear {}: {e}", parent.display()))
            })?;
        }
        let temp_arg: String = temp.to_string_lossy().into_owned();
        // Limpia registros de worktrees temporales huérfanos de merges anteriores.
        if let Err(error) = git(repo, &["worktree", "prune"]).await {
            tracing::warn!("no se pudo ejecutar git worktree prune antes del merge: {error}");
        }
        git(repo, &["worktree", "add", &temp_arg, base_branch]).await?;
        let outcome: Result<MergeOutcome, TaskError> = self.merge_in(&temp, branch, message).await;
        // La limpieza nunca cambia el resultado del merge.
        if let Err(error) = git(repo, &["worktree", "remove", "--force", &temp_arg]).await {
            tracing::warn!(
                "no se pudo eliminar el worktree temporal {}: {error}; se intenta borrar la carpeta",
                temp.display()
            );
            if let Err(error) = tokio::fs::remove_dir_all(&temp).await {
                tracing::warn!("no se pudo borrar {}: {error}", temp.display());
            }
            if let Err(error) = git(repo, &["worktree", "prune"]).await {
                tracing::warn!("no se pudo ejecutar git worktree prune tras el merge: {error}");
            }
        }
        outcome
    }

    async fn remove(&self, repo: &Path, path: &Path, branch: &str) -> Result<(), TaskError> {
        if path.exists() {
            let path_arg: String = path.to_string_lossy().into_owned();
            git(repo, &["worktree", "remove", "--force", &path_arg]).await?;
        } else {
            git(repo, &["worktree", "prune"]).await?;
        }
        let reference: String = format!("refs/heads/{branch}");
        let exists: bool = git_raw(repo, &["rev-parse", "--verify", "--quiet", &reference])
            .await?
            .success;
        if exists {
            git(repo, &["branch", "-D", branch]).await?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn finds_worktree_holding_branch() {
        let porcelain: &str = "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\nworktree /wt/x\nHEAD def\nbranch refs/heads/pi/x\n";
        assert_eq!(
            worktree_with_branch(porcelain, "main"),
            Some(PathBuf::from("/repo"))
        );
        assert_eq!(
            worktree_with_branch(porcelain, "pi/x"),
            Some(PathBuf::from("/wt/x"))
        );
        assert_eq!(worktree_with_branch(porcelain, "develop"), None);
    }
}
