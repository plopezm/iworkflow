use std::path::Path;

use crate::core::git::{GitOutput, run_git_raw};
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
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

    async fn publish_branch(&self, repo: &Path, branch: &str) -> Result<(), CodeHostError> {
        let refspec: String = format!("refs/heads/{branch}:refs/heads/{branch}");
        let output: GitOutput = run_git_raw(repo, &["push", ORIGIN, &refspec])
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
}
