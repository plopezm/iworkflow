use std::path::Path;

use base64::Engine;
use base64::engine::general_purpose::STANDARD;

use crate::core::git::{GitOutput, run_git_raw, run_git_raw_env};
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::ports::outbound::git_remote_port::GitRemotePort;

const ORIGIN: &str = "origin";

/// Lee y publica en los remotos con la CLI de `git`.
#[derive(Debug, Default)]
pub struct GitCliRemoteAdapter;

impl GitCliRemoteAdapter {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait::async_trait]
impl GitRemotePort for GitCliRemoteAdapter {
    async fn origin_url(&self, repo: &Path) -> Result<String, CodeHostError> {
        let output: GitOutput = run_git_raw(repo, &["remote", "get-url", ORIGIN])
            .await
            .map_err(|e| CodeHostError::Git(e.to_string()))?;
        let url: String = output.stdout.trim().to_string();
        if output.success && !url.is_empty() {
            Ok(url)
        } else {
            Err(CodeHostError::NoOrigin(repo.display().to_string()))
        }
    }

    async fn publish_branch(
        &self,
        repo: &Path,
        branch: &str,
        settings: &CodeHostSettings,
        project: &str,
    ) -> Result<(), CodeHostError> {
        let refspec: String = format!("refs/heads/{branch}:refs/heads/{branch}");
        let url: String = settings.push_url(project);
        let output: GitOutput =
            run_git_raw_env(repo, &["push", &url, &refspec], &token_config(settings))
                .await
                .map_err(|e| CodeHostError::Git(e.to_string()))?;
        if output.success {
            Ok(())
        } else {
            Err(CodeHostError::BranchPublishFailed {
                branch: branch.to_string(),
                reason: output.stderr,
            })
        }
    }

    async fn ensure_commits_ahead(
        &self,
        repo: &Path,
        branch: &str,
        base: &str,
    ) -> Result<bool, CodeHostError> {
        let range: String = format!("{base}..{branch}");
        let count: GitOutput = run_git_raw(repo, &["rev-list", "--count", &range])
            .await
            .map_err(|e| CodeHostError::Git(e.to_string()))?;
        // Si la base no se resuelve en local, que decida el proveedor.
        if !count.success || count.stdout.trim() != "0" {
            return Ok(false);
        }
        let head: String = rev_parse(repo, &format!("refs/heads/{branch}")).await?;
        let tree: String = rev_parse(repo, &format!("refs/heads/{branch}^{{tree}}")).await?;
        let commit: GitOutput = run_git_raw_env(
            repo,
            &[
                "commit-tree",
                &tree,
                "-p",
                &head,
                "-m",
                EMPTY_COMMIT_MESSAGE,
            ],
            &fallback_identity(repo).await?,
        )
        .await
        .map_err(|e| CodeHostError::Git(e.to_string()))?;
        let new_head: String = commit.stdout.trim().to_string();
        if !commit.success || new_head.is_empty() {
            return Err(CodeHostError::Git(commit.stderr));
        }
        let update_ref: String = format!("refs/heads/{branch}");
        let moved: GitOutput = run_git_raw(repo, &["update-ref", &update_ref, &new_head, &head])
            .await
            .map_err(|e| CodeHostError::Git(e.to_string()))?;
        if moved.success {
            Ok(true)
        } else {
            Err(CodeHostError::Git(moved.stderr))
        }
    }
}

const EMPTY_COMMIT_MESSAGE: &str = "chore: empty commit to open the PR/MR";

async fn rev_parse(repo: &Path, spec: &str) -> Result<String, CodeHostError> {
    let output: GitOutput = run_git_raw(repo, &["rev-parse", "--verify", spec])
        .await
        .map_err(|e| CodeHostError::Git(e.to_string()))?;
    let value: String = output.stdout.trim().to_string();
    if output.success && !value.is_empty() {
        Ok(value)
    } else {
        Err(CodeHostError::Git(output.stderr))
    }
}

/// Identidad de respaldo solo si git no tiene una configurada (`commit-tree` la exige).
async fn fallback_identity(repo: &Path) -> Result<Vec<(String, String)>, CodeHostError> {
    let ident: GitOutput = run_git_raw(repo, &["var", "GIT_COMMITTER_IDENT"])
        .await
        .map_err(|e| CodeHostError::Git(e.to_string()))?;
    if ident.success {
        return Ok(Vec::new());
    }
    Ok(["AUTHOR", "COMMITTER"]
        .iter()
        .flat_map(|who: &&str| {
            [
                (format!("GIT_{who}_NAME"), "iworkflow".to_string()),
                (
                    format!("GIT_{who}_EMAIL"),
                    "iworkflow@localhost".to_string(),
                ),
            ]
        })
        .collect())
}

/// Configuración de git por entorno (`GIT_CONFIG_COUNT/KEY_n/VALUE_n`): a diferencia de `-c`,
/// el token no queda en la línea de comandos. La cabecera solo se envía a la instancia
/// configurada (`http.<url>.extraHeader`) y se anula cualquier credential helper del usuario.
fn token_config(settings: &CodeHostSettings) -> Vec<(String, String)> {
    let credentials: String =
        STANDARD.encode(format!("{}:{}", settings.git_username(), settings.token()));
    let scope: String = format!("http.{}/", settings.base_url());
    let mut entries: Vec<(String, String)> = vec![
        (
            format!("{scope}.extraHeader"),
            format!("Authorization: Basic {credentials}"),
        ),
        ("credential.helper".to_string(), String::new()),
    ];
    if settings.insecure_tls {
        entries.push((format!("{scope}.sslVerify"), "false".to_string()));
    }
    let mut envs: Vec<(String, String)> =
        vec![("GIT_CONFIG_COUNT".to_string(), entries.len().to_string())];
    for (index, (key, value)) in entries.into_iter().enumerate() {
        envs.push((format!("GIT_CONFIG_KEY_{index}"), key));
        envs.push((format!("GIT_CONFIG_VALUE_{index}"), value));
    }
    envs
}
