//! Ejecución de la CLI de `git` sin conocimiento de dominio.

use std::path::Path;
use std::process::{Output, Stdio};

use tokio::io::AsyncWriteExt;
use tokio::process::{Child, Command};

#[derive(Debug, thiserror::Error)]
pub enum GitCommandError {
    #[error("no se pudo ejecutar git: {0}")]
    Spawn(#[from] std::io::Error),
    #[error("git {args} falló ({code:?}): {stderr}")]
    Failed {
        args: String,
        code: Option<i32>,
        stderr: String,
    },
}

/// Resultado crudo de un comando git (para casos en los que un código distinto de 0 es esperado).
#[derive(Debug, Clone)]
pub struct GitOutput {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
}

/// Ejecuta `git -C <dir> <args>` y devuelve la salida sin tratar los fallos como error.
pub async fn run_git_raw(dir: &Path, args: &[&str]) -> Result<GitOutput, GitCommandError> {
    run_git_raw_env(dir, args, &[]).await
}

/// Como [`run_git_raw`], con variables de entorno extra solo para este proceso. Sirve para
/// pasar configuración sensible (`GIT_CONFIG_*`) sin que aparezca en la línea de comandos.
pub async fn run_git_raw_env(
    dir: &Path,
    args: &[&str],
    envs: &[(String, String)],
) -> Result<GitOutput, GitCommandError> {
    let output: Output = Command::new("git")
        .arg("-C")
        .arg(dir)
        .args(args)
        .env("GIT_TERMINAL_PROMPT", "0")
        .envs(envs.iter().map(|(k, v): &(String, String)| (k, v)))
        .kill_on_drop(true)
        .output()
        .await?;
    Ok(GitOutput {
        success: output.status.success(),
        stdout: String::from_utf8_lossy(&output.stdout).into_owned(),
        stderr: String::from_utf8_lossy(&output.stderr).trim().to_string(),
    })
}

/// Ejecuta `git -C <dir> <args>` y devuelve stdout recortado; un código distinto de 0 es error.
pub async fn run_git(dir: &Path, args: &[&str]) -> Result<String, GitCommandError> {
    let output: GitOutput = run_git_raw(dir, args).await?;
    if output.success {
        Ok(output.stdout.trim().to_string())
    } else {
        Err(GitCommandError::Failed {
            args: args.join(" "),
            code: None,
            stderr: output.stderr,
        })
    }
}

/// Ejecuta `git -C <dir> <args>` escribiendo `stdin_data` en la entrada estándar del proceso.
/// Necesario para comandos de plumbing como `hash-object --stdin` que no tocan el índice.
pub async fn run_git_with_stdin(
    dir: &Path,
    args: &[&str],
    stdin_data: &[u8],
) -> Result<String, GitCommandError> {
    let mut child: Child = Command::new("git")
        .arg("-C")
        .arg(dir)
        .args(args)
        .env("GIT_TERMINAL_PROMPT", "0")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .kill_on_drop(true)
        .spawn()?;

    if let Some(mut child_stdin) = child.stdin.take() {
        child_stdin.write_all(stdin_data).await?;
    }

    let output: Output = child.wait_with_output().await?;
    if output.status.success() {
        Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
    } else {
        Err(GitCommandError::Failed {
            args: args.join(" "),
            code: output.status.code(),
            stderr: String::from_utf8_lossy(&output.stderr).trim().to_string(),
        })
    }
}

/// Argumentos de git que habilitan las rutas largas (>260 caracteres) en Windows.
const LONG_PATHS_ARGS: [&str; 4] = ["config", "--global", "core.longpaths", "true"];

/// Habilita `core.longpaths` en la configuración global de git. Solo aplica en Windows, donde
/// crear worktrees con rutas largas falla si no está activo; en otras plataformas no hace nada.
pub async fn enable_long_paths() -> Result<(), GitCommandError> {
    if cfg!(windows) {
        run_long_paths_config(None).await?;
    }
    Ok(())
}

/// Ejecuta la configuración; `global_config` redirige el fichero global (`GIT_CONFIG_GLOBAL`).
async fn run_long_paths_config(global_config: Option<&Path>) -> Result<(), GitCommandError> {
    let mut command: Command = Command::new("git");
    command
        .args(LONG_PATHS_ARGS)
        .env("GIT_TERMINAL_PROMPT", "0")
        .kill_on_drop(true);
    if let Some(path) = global_config {
        command.env("GIT_CONFIG_GLOBAL", path);
    }
    let output: Output = command.output().await?;
    if output.status.success() {
        Ok(())
    } else {
        Err(GitCommandError::Failed {
            args: LONG_PATHS_ARGS.join(" "),
            code: output.status.code(),
            stderr: String::from_utf8_lossy(&output.stderr).trim().to_string(),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn long_paths_config_sets_core_longpaths_in_global_file() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let global: std::path::PathBuf = dir.path().join("gitconfig");
        run_long_paths_config(Some(&global)).await.expect("config");
        let content: String = std::fs::read_to_string(&global).expect("read");
        assert!(content.contains("longpaths = true"), "{content}");
    }

    #[tokio::test]
    async fn run_git_returns_error_outside_repository() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let result: Result<String, GitCommandError> =
            run_git(dir.path(), &["rev-parse", "--show-toplevel"]).await;
        assert!(matches!(result, Err(GitCommandError::Failed { .. })));
    }

    #[tokio::test]
    async fn run_git_returns_trimmed_stdout() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        run_git(dir.path(), &["init", "-q"]).await.expect("init");
        let top: String = run_git(dir.path(), &["rev-parse", "--show-toplevel"])
            .await
            .expect("toplevel");
        let expected: std::path::PathBuf = dir.path().canonicalize().expect("canonical");
        assert_eq!(std::path::PathBuf::from(top), expected);
    }
}
