use std::path::{Path, PathBuf};

use crate::core::git::{GitCommandError, GitOutput, run_git_raw};
use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::install_outcome::InstallOutcome;
use crate::modules::agents_files::domain::ports::outbound::agents_file_installer_port::AgentsFileInstallerPort;

const TARGET_FILE_NAME: &str = "AGENTS.md";
const COMMIT_MESSAGE: &str = "docs: add AGENTS.md";
const FALLBACK_USER_NAME: &str = "pi-workflow";
const FALLBACK_USER_EMAIL: &str = "pi-workflow@localhost";

/// Escribe `AGENTS.md` en una carpeta y, si es un repositorio git, lo commitea solo a él.
#[derive(Default)]
pub struct GitAgentsFileInstallerAdapter;

impl GitAgentsFileInstallerAdapter {
    pub fn new() -> Self {
        Self
    }
}

async fn git_config_is_set(dir: &Path, key: &str) -> Result<bool, GitCommandError> {
    let output: GitOutput = run_git_raw(dir, &["config", key]).await?;
    Ok(output.success && !output.stdout.trim().is_empty())
}

async fn identity_args(dir: &Path) -> Result<Vec<String>, GitCommandError> {
    if git_config_is_set(dir, "user.name").await? && git_config_is_set(dir, "user.email").await? {
        return Ok(Vec::new());
    }
    Ok(vec![
        "-c".to_string(),
        format!("user.name={FALLBACK_USER_NAME}"),
        "-c".to_string(),
        format!("user.email={FALLBACK_USER_EMAIL}"),
    ])
}

fn git_error(error: GitCommandError) -> AgentsFileError {
    AgentsFileError::Install(error.to_string())
}

/// `Ok(true)` si el commit se creó; `Ok(false)` si no había cambios que commitear.
async fn commit_target(dir: &Path) -> Result<bool, AgentsFileError> {
    let add: GitOutput = run_git_raw(dir, &["add", "--", TARGET_FILE_NAME])
        .await
        .map_err(git_error)?;
    if !add.success {
        return Err(AgentsFileError::Install(add.stderr));
    }
    let staged: GitOutput = run_git_raw(
        dir,
        &["diff", "--cached", "--quiet", "--", TARGET_FILE_NAME],
    )
    .await
    .map_err(git_error)?;
    if staged.success {
        return Ok(false);
    }
    let identity: Vec<String> = identity_args(dir).await.map_err(git_error)?;
    let mut args: Vec<&str> = identity.iter().map(String::as_str).collect();
    args.extend([
        "commit",
        "-q",
        "--only",
        "-m",
        COMMIT_MESSAGE,
        "--",
        TARGET_FILE_NAME,
    ]);
    let commit: GitOutput = run_git_raw(dir, &args).await.map_err(git_error)?;
    if commit.success {
        Ok(true)
    } else {
        Err(AgentsFileError::Install(commit.stderr))
    }
}

#[async_trait::async_trait]
impl AgentsFileInstallerPort for GitAgentsFileInstallerAdapter {
    async fn install(
        &self,
        content: &[u8],
        dir: &Path,
        overwrite: bool,
    ) -> Result<InstallOutcome, AgentsFileError> {
        let is_dir: bool = tokio::fs::metadata(dir)
            .await
            .map(|m: std::fs::Metadata| m.is_dir())
            .unwrap_or(false);
        if !is_dir {
            return Err(AgentsFileError::Install(format!(
                "la carpeta no existe: {}",
                dir.display()
            )));
        }
        let target: PathBuf = dir.join(TARGET_FILE_NAME);
        let exists: bool = tokio::fs::try_exists(&target)
            .await
            .map_err(|e: std::io::Error| AgentsFileError::Install(e.to_string()))?;
        if exists && !overwrite {
            return Err(AgentsFileError::TargetExists(target.display().to_string()));
        }
        tokio::fs::write(&target, content)
            .await
            .map_err(|e: std::io::Error| {
                AgentsFileError::Install(format!("no se pudo escribir {}: {e}", target.display()))
            })?;

        let inside_repo: bool = match run_git_raw(dir, &["rev-parse", "--show-toplevel"]).await {
            Ok(output) => output.success,
            Err(error) => {
                tracing::warn!("no se pudo ejecutar git en {}: {error}", dir.display());
                false
            }
        };
        if !inside_repo {
            return Ok(InstallOutcome { committed: false });
        }
        match commit_target(dir).await {
            Ok(committed) => Ok(InstallOutcome { committed }),
            Err(error) => {
                tracing::warn!(
                    "AGENTS.md escrito en {} pero no commiteado: {error}",
                    dir.display()
                );
                Ok(InstallOutcome { committed: false })
            }
        }
    }
}
