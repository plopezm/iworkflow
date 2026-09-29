use std::path::{Path, PathBuf};
use std::process::Output;

use tokio::process::Command;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
use crate::modules::agent_settings::domain::ports::outbound::profile_pin_port::ProfilePinPort;

const PIN_RELATIVE_PATH: &str = "gentle-ai/profile-pin.json";
const TMP_SUFFIX: &str = "tmp";

/// Escribe/borra el pin local de perfil en `<git-common-dir>/gentle-ai/profile-pin.json`,
/// resuelto vía `git rev-parse --git-common-dir` desde el worktree de la tarea.
pub struct GitProfilePinAdapter;

impl GitProfilePinAdapter {
    pub fn new() -> Self {
        Self
    }
}

impl Default for GitProfilePinAdapter {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait::async_trait]
impl ProfilePinPort for GitProfilePinAdapter {
    async fn apply<'a>(
        &self,
        repo_dir: &'a Path,
        profile: Option<&'a ProfileName>,
    ) -> Result<(), AgentSettingsError> {
        let common_dir: PathBuf = git_common_dir(repo_dir).await?;
        let pin_path: PathBuf = common_dir.join(PIN_RELATIVE_PATH);
        match profile {
            Some(name) => write_pin(&pin_path, name).await,
            None => remove_pin(&pin_path).await,
        }
    }
}

async fn git_common_dir(repo_dir: &Path) -> Result<PathBuf, AgentSettingsError> {
    let output: Output = Command::new("git")
        .arg("-C")
        .arg(repo_dir)
        .args(["rev-parse", "--git-common-dir"])
        .output()
        .await
        .map_err(|e| AgentSettingsError::Io(e.to_string()))?;
    if !output.status.success() {
        return Err(AgentSettingsError::Io(format!(
            "git rev-parse --git-common-dir falló: {}",
            String::from_utf8_lossy(&output.stderr)
        )));
    }
    let raw: String = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let path: PathBuf = PathBuf::from(raw);
    Ok(if path.is_absolute() {
        path
    } else {
        repo_dir.join(path)
    })
}

fn pin_content(profile: &ProfileName) -> String {
    format!(
        "{{\n  \"kind\": \"gentle-pi.agent_model_profile_pin\",\n  \"version\": 1,\n  \"profile\": \"{}\"\n}}\n",
        profile.as_str()
    )
}

async fn write_pin(pin_path: &Path, profile: &ProfileName) -> Result<(), AgentSettingsError> {
    if let Some(parent) = pin_path.parent() {
        tokio::fs::create_dir_all(parent)
            .await
            .map_err(|e| AgentSettingsError::Io(e.to_string()))?;
    }
    let tmp_path: PathBuf = pin_path.with_extension(TMP_SUFFIX);
    tokio::fs::write(&tmp_path, pin_content(profile))
        .await
        .map_err(|e| AgentSettingsError::Io(e.to_string()))?;
    tokio::fs::rename(&tmp_path, pin_path)
        .await
        .map_err(|e| AgentSettingsError::Io(e.to_string()))?;
    Ok(())
}

async fn remove_pin(pin_path: &Path) -> Result<(), AgentSettingsError> {
    match tokio::fs::remove_file(pin_path).await {
        Ok(()) => Ok(()),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(e) => Err(AgentSettingsError::Io(e.to_string())),
    }
}
