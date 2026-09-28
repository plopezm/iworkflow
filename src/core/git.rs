//! Ejecución de la CLI de `git` sin conocimiento de dominio.

use std::path::Path;
use std::process::Output;

use tokio::process::Command;

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
    let output: Output = Command::new("git")
        .arg("-C")
        .arg(dir)
        .args(args)
        .env("GIT_TERMINAL_PROMPT", "0")
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

#[cfg(test)]
mod tests {
    use super::*;

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
