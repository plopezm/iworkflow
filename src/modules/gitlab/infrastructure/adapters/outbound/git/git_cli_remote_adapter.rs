use std::path::Path;

use crate::core::git::{GitOutput, run_git_raw};
use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::ports::outbound::git_remote_port::GitRemotePort;

const ORIGIN: &str = "origin";

/// Lee los remotos con la CLI de `git`. Solo lectura: nunca publica ramas.
#[derive(Debug, Default)]
pub struct GitCliRemoteAdapter;

impl GitCliRemoteAdapter {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait::async_trait]
impl GitRemotePort for GitCliRemoteAdapter {
    async fn origin_url(&self, repo: &Path) -> Result<String, GitLabError> {
        let output: GitOutput = run_git_raw(repo, &["remote", "get-url", ORIGIN])
            .await
            .map_err(|e| GitLabError::Git(e.to_string()))?;
        let url: String = output.stdout.trim().to_string();
        if output.success && !url.is_empty() {
            Ok(url)
        } else {
            Err(GitLabError::NoOrigin(repo.display().to_string()))
        }
    }
}
