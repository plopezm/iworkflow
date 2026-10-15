use std::path::{Path, PathBuf};

use crate::core::git::{GitCommandError, GitOutput, run_git, run_git_raw};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::branch_option::BranchOption;
use crate::modules::tasks::domain::models::task::{is_task_branch, slugify};
use crate::modules::tasks::domain::models::worktree::{DiffRange, MergeOutcome, TaskDiff};
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Tamaño máximo del parche devuelto al cliente.
const MAX_PATCH_BYTES: usize = 1024 * 1024;
/// Identidad usada solo si el repositorio no tiene una configurada.
const FALLBACK_USER_NAME: &str = "iworkflow";
const FALLBACK_USER_EMAIL: &str = "iworkflow@localhost";
/// Subcarpeta (dentro de la raíz de worktrees) para merges temporales.
const MERGE_DIR: &str = ".merge";
/// Subcarpeta (dentro de la raíz de worktrees) donde se aparcan los worktrees pendientes de
/// borrado; evita esperar al borrado de `target/` (decenas de GB) dentro de la petición.
const TRASH_DIR: &str = ".trash";

/// Worktrees gestionados con la CLI de git bajo una carpeta raíz.
pub struct GitCliWorktreeAdapter {
    root: PathBuf,
    /// Serializa los merges: todos escriben en la misma rama base y compiten por `index.lock`.
    merge_lock: tokio::sync::Mutex<()>,
    /// Borrados en segundo plano de la papelera; se conservan para poder esperarlos.
    trash_tasks: std::sync::Mutex<Vec<tokio::task::JoinHandle<()>>>,
}

impl GitCliWorktreeAdapter {
    pub fn new(root: PathBuf) -> Self {
        Self {
            root,
            merge_lock: tokio::sync::Mutex::new(()),
            trash_tasks: std::sync::Mutex::new(Vec::new()),
        }
    }

    /// Espera a que terminen los borrados en segundo plano lanzados hasta ahora.
    pub async fn wait_for_cleanup(&self) {
        let handles: Vec<tokio::task::JoinHandle<()>> = {
            let mut guard = self
                .trash_tasks
                .lock()
                .unwrap_or_else(std::sync::PoisonError::into_inner);
            std::mem::take(&mut *guard)
        };
        for handle in handles {
            if let Err(error) = handle.await {
                tracing::warn!("el borrado en segundo plano de worktrees falló: {error}");
            }
        }
    }

    /// Aparca el worktree en la papelera (renombrado atómico), lo desregistra en git y lanza su
    /// borrado en segundo plano. Devuelve `false` si no aplica (no registrado, bloqueado, fuera
    /// de la raíz, rename imposible...) y hay que usar el borrado síncrono.
    async fn defer_removal(&self, repo: &Path, path: &Path) -> bool {
        let Ok(porcelain) = git(repo, &["worktree", "list", "--porcelain"]).await else {
            return false;
        };
        if is_dead_worktree(&porcelain, path) || is_locked_worktree(&porcelain, path) {
            return false;
        }
        if self.ensure_inside_root(path).await.is_err() {
            return false;
        }
        let trash: PathBuf = self.root.join(TRASH_DIR);
        let parked: PathBuf = trash.join(uuid::Uuid::new_v4().to_string());
        if tokio::fs::create_dir_all(&trash).await.is_err() {
            return false;
        }
        if let Err(error) = tokio::fs::rename(path, &parked).await {
            tracing::warn!(
                "no se pudo aparcar {} en la papelera ({error}); se borra de forma síncrona",
                path.display()
            );
            return false;
        }
        // El gitdir apunta ya a una carpeta inexistente: prune elimina el registro.
        if let Err(error) = git(repo, &["worktree", "prune"]).await {
            tracing::warn!(
                "no se pudo ejecutar git worktree prune tras aparcar el worktree: {error}"
            );
        }
        let handle: tokio::task::JoinHandle<()> = tokio::spawn(purge_trash(trash));
        let mut guard = self
            .trash_tasks
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner);
        guard.retain(|task| !task.is_finished());
        guard.push(handle);
        true
    }

    /// Borrado síncrono: `git worktree remove --force` y, si git ya no lo reconoce, la carpeta.
    async fn remove_now(&self, repo: &Path, path: &Path) -> Result<(), TaskError> {
        let path_arg: String = path.to_string_lossy().into_owned();
        if let Err(error) = git(repo, &["worktree", "remove", "--force", &path_arg]).await {
            // Solo se descarta por borrado si git ya no considera la carpeta un worktree
            // utilizable; cualquier otro fallo (bloqueado, permisos...) se propaga.
            let porcelain: String = git(repo, &["worktree", "list", "--porcelain"])
                .await
                .map_err(|_| error.clone())?;
            if !is_dead_worktree(&porcelain, path) {
                return Err(error);
            }
            tracing::warn!(
                "{} ya no es un worktree válido ({error}); se borra la carpeta",
                path.display()
            );
            self.ensure_inside_root(path).await?;
            tokio::fs::remove_dir_all(path).await.map_err(|e| {
                TaskError::Git(format!("no se pudo borrar {}: {e}", path.display()))
            })?;
        }
        Ok(())
    }

    /// Comprueba que `path` queda estrictamente dentro de la raíz de worktrees una vez resueltos
    /// los enlaces simbólicos (ni vacía, ni la propia raíz, ni un escape por symlink).
    async fn ensure_inside_root(&self, path: &Path) -> Result<(), TaskError> {
        let unsafe_path = || TaskError::UnsafeWorktreePath(path.display().to_string());
        if path.as_os_str().is_empty() {
            return Err(unsafe_path());
        }
        let canonical_root: PathBuf = tokio::fs::canonicalize(&self.root)
            .await
            .map_err(|_| unsafe_path())?;
        let canonical_path: PathBuf = tokio::fs::canonicalize(path)
            .await
            .map_err(|_| unsafe_path())?;
        if canonical_path == canonical_root || !canonical_path.starts_with(&canonical_root) {
            return Err(unsafe_path());
        }
        Ok(())
    }
}

fn git_error(error: GitCommandError) -> TaskError {
    TaskError::Git(error.to_string())
}

async fn git(dir: &Path, args: &[&str]) -> Result<String, TaskError> {
    run_git(dir, args).await.map_err(git_error)
}

async fn ref_exists(repo: &Path, reference: &str) -> Result<bool, TaskError> {
    Ok(
        git_raw(repo, &["rev-parse", "--verify", "--quiet", reference])
            .await?
            .success,
    )
}

async fn git_raw(dir: &Path, args: &[&str]) -> Result<GitOutput, TaskError> {
    run_git_raw(dir, args).await.map_err(git_error)
}

/// Construye un `TaskDiff` con `git diff <diff_spec>` y `git log <log_range>`; el parche se
/// trunca a `MAX_PATCH_BYTES`.
async fn build_diff(repo: &Path, diff_spec: &str, log_range: &str) -> Result<TaskDiff, TaskError> {
    let stat: String = git(repo, &["diff", "--stat", diff_spec]).await?;
    let full_patch: String = git(repo, &["diff", diff_spec]).await?;
    let commits: Vec<String> = git(repo, &["log", "--oneline", log_range])
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

/// `true` si `path` no figura en `git worktree list --porcelain` o figura como `prunable`
/// (su gitdir ya no existe), es decir, git no puede eliminarlo como worktree.
fn is_dead_worktree(porcelain: &str, path: &Path) -> bool {
    let canonical: PathBuf = path.canonicalize().unwrap_or_else(|_| path.to_path_buf());
    porcelain
        .split("\n\n")
        .find(|block| {
            block
                .lines()
                .next()
                .and_then(|line| line.strip_prefix("worktree "))
                .is_some_and(|listed| Path::new(listed) == path || Path::new(listed) == canonical)
        })
        .is_none_or(|block| block.lines().any(|line| line.starts_with("prunable")))
}

/// `true` si `path` figura en `git worktree list --porcelain` como `locked`.
fn is_locked_worktree(porcelain: &str, path: &Path) -> bool {
    let canonical: PathBuf = path.canonicalize().unwrap_or_else(|_| path.to_path_buf());
    porcelain.split("\n\n").any(|block| {
        let listed_here: bool = block
            .lines()
            .next()
            .and_then(|line| line.strip_prefix("worktree "))
            .is_some_and(|listed| Path::new(listed) == path || Path::new(listed) == canonical);
        listed_here && block.lines().any(|line| line.starts_with("locked"))
    })
}

/// Borra todo lo que haya en la papelera. Los errores se registran: no hay nadie esperando.
async fn purge_trash(trash: PathBuf) {
    let mut entries: tokio::fs::ReadDir = match tokio::fs::read_dir(&trash).await {
        Ok(entries) => entries,
        Err(error) => {
            tracing::warn!("no se pudo leer la papelera {}: {error}", trash.display());
            return;
        }
    };
    while let Ok(Some(entry)) = entries.next_entry().await {
        let entry_path: PathBuf = entry.path();
        // NotFound: otro borrado en segundo plano se adelantó; no es un error.
        if let Err(error) = tokio::fs::remove_dir_all(&entry_path).await
            && error.kind() != std::io::ErrorKind::NotFound
        {
            tracing::warn!("no se pudo borrar {}: {error}", entry_path.display());
        }
    }
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

const LOCAL_REFS_PREFIX: &str = "refs/heads/";
const REMOTE_REFS_PREFIX: &str = "refs/remotes/";

/// Ramas de un `git for-each-ref --format=%(refname)`: locales primero, luego remotas, cada
/// grupo ordenado; sin las ramas de tareas ni los `<remoto>/HEAD`.
fn parse_branches(refs: &str, current: &str) -> Vec<BranchOption> {
    let mut local: Vec<BranchOption> = Vec::new();
    let mut remote: Vec<BranchOption> = Vec::new();
    for reference in refs.lines().map(str::trim) {
        if let Some(name) = reference.strip_prefix(LOCAL_REFS_PREFIX) {
            if !is_task_branch(name) {
                local.push(BranchOption {
                    name: name.to_string(),
                    remote: false,
                    current: name == current,
                });
            }
        } else if let Some(name) = reference.strip_prefix(REMOTE_REFS_PREFIX)
            && !name.ends_with("/HEAD")
        {
            remote.push(BranchOption {
                name: name.to_string(),
                remote: true,
                current: false,
            });
        }
    }
    local.sort_by(|a: &BranchOption, b: &BranchOption| a.name.cmp(&b.name));
    remote.sort_by(|a: &BranchOption, b: &BranchOption| a.name.cmp(&b.name));
    local.extend(remote);
    local
}

/// Nombre local de una rama remota (`origin/develop` -> `develop`) dado el listado de remotos.
fn local_name_of_remote<'a>(base_branch: &'a str, remotes: &str) -> Option<&'a str> {
    remotes
        .lines()
        .map(str::trim)
        .filter(|remote: &&str| !remote.is_empty())
        .find_map(|remote: &str| {
            base_branch
                .strip_prefix(remote)
                .and_then(|rest: &str| rest.strip_prefix('/'))
        })
        .filter(|local: &&str| !local.is_empty())
}

impl GitCliWorktreeAdapter {
    /// Rama local donde integrar una tarea cuya base es `base_branch`. Si la base es una rama de
    /// seguimiento remoto sin homónima local, se crea la local siguiendo a la remota.
    async fn merge_target(&self, repo: &Path, base_branch: &str) -> Result<String, TaskError> {
        if ref_exists(repo, &format!("{LOCAL_REFS_PREFIX}{base_branch}")).await? {
            return Ok(base_branch.to_string());
        }
        if !ref_exists(repo, &format!("{REMOTE_REFS_PREFIX}{base_branch}")).await? {
            return Ok(base_branch.to_string());
        }
        let remotes: String = git(repo, &["remote"]).await?;
        let Some(local) = local_name_of_remote(base_branch, &remotes) else {
            return Ok(base_branch.to_string());
        };
        if !ref_exists(repo, &format!("{LOCAL_REFS_PREFIX}{local}")).await? {
            git(repo, &["branch", "--track", local, base_branch]).await?;
        }
        Ok(local.to_string())
    }

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

    async fn list_branches(&self, repo: &Path) -> Result<Vec<BranchOption>, TaskError> {
        let refs: String = git(
            repo,
            &[
                "for-each-ref",
                "--format=%(refname)",
                LOCAL_REFS_PREFIX,
                REMOTE_REFS_PREFIX,
            ],
        )
        .await?;
        // Vacío con HEAD desacoplado: entonces ninguna rama es la actual.
        let current: String = git(repo, &["branch", "--show-current"]).await?;
        Ok(parse_branches(&refs, current.trim()))
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
        let diff_spec: String = format!("{base_branch}...{branch}");
        let log_range: String = format!("{base_branch}..{branch}");
        build_diff(repo, &diff_spec, &log_range).await
    }

    async fn diff_fingerprint(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
    ) -> Result<String, TaskError> {
        // `base...branch` solo depende de las puntas de ambas ramas (merge-base incluido).
        let shas: String = git(repo, &["rev-parse", base_branch, branch]).await?;
        Ok(shas.split_whitespace().collect::<Vec<&str>>().join(":"))
    }

    async fn pin_range(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        pin_ref: &str,
    ) -> Result<DiffRange, TaskError> {
        let base_sha: String = git(repo, &["merge-base", base_branch, branch])
            .await?
            .trim()
            .to_string();
        let head_sha: String = git(repo, &["rev-parse", branch]).await?.trim().to_string();
        git(repo, &["update-ref", pin_ref, &head_sha]).await?;
        Ok(DiffRange { base_sha, head_sha })
    }

    async fn diff_range(&self, repo: &Path, range: &DiffRange) -> Result<TaskDiff, TaskError> {
        let diff_spec: String = format!("{}..{}", range.base_sha, range.head_sha);
        build_diff(repo, &diff_spec, &diff_spec).await
    }

    async fn merge(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        message: &str,
    ) -> Result<MergeOutcome, TaskError> {
        let _guard: tokio::sync::MutexGuard<'_, ()> = self.merge_lock.lock().await;
        let target_branch: String = self.merge_target(repo, base_branch).await?;
        let base_branch: &str = &target_branch;
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
        if path.exists() && !self.defer_removal(repo, path).await {
            self.remove_now(repo, path).await?;
        }
        if let Err(error) = git(repo, &["worktree", "prune"]).await {
            tracing::warn!("no se pudo ejecutar git worktree prune: {error}");
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
    fn dead_worktree_is_unregistered_or_prunable_only() {
        let porcelain: &str = "worktree /repo\nHEAD a\nbranch refs/heads/main\n\nworktree /wt/ok\nHEAD b\nbranch refs/heads/x\n\nworktree /wt/gone\nHEAD c\nbranch refs/heads/y\nprunable gitdir file points to non-existent location\n\nworktree /wt/locked\nHEAD d\nbranch refs/heads/z\nlocked\n";
        assert!(is_dead_worktree(porcelain, Path::new("/wt/unknown")));
        assert!(is_dead_worktree(porcelain, Path::new("/wt/gone")));
        assert!(!is_dead_worktree(porcelain, Path::new("/wt/ok")));
        assert!(!is_dead_worktree(porcelain, Path::new("/wt/locked")));
    }

    #[test]
    fn lists_local_then_remote_branches_without_task_branches_or_remote_head() {
        let refs: &str = "refs/heads/main\nrefs/heads/pi/fix-1\nrefs/heads/develop\nrefs/remotes/origin/HEAD\nrefs/remotes/origin/main\nrefs/remotes/origin/develop\nrefs/remotes/up/pi/x\n";
        let names: Vec<(String, bool)> = parse_branches(refs, "main")
            .into_iter()
            .map(|b: BranchOption| (b.name, b.remote))
            .collect();
        let current: Vec<String> = parse_branches(refs, "main")
            .into_iter()
            .filter(|b: &BranchOption| b.current)
            .map(|b: BranchOption| b.name)
            .collect();
        assert_eq!(current, vec!["main".to_string()]);
        assert_eq!(
            names,
            vec![
                ("develop".to_string(), false),
                ("main".to_string(), false),
                ("origin/develop".to_string(), true),
                ("origin/main".to_string(), true),
                ("up/pi/x".to_string(), true),
            ]
        );
    }

    #[test]
    fn strips_the_remote_name_from_a_remote_branch() {
        assert_eq!(
            local_name_of_remote("origin/feature/x", "upstream\norigin\n"),
            Some("feature/x")
        );
        assert_eq!(local_name_of_remote("origin", "origin\n"), None);
        assert_eq!(local_name_of_remote("develop", "origin\n"), None);
    }

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
