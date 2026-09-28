use std::path::{Path, PathBuf};

use crate::core::git::{GitCommandError, GitOutput, run_git, run_git_raw};
use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::ports::outbound::git_repository_port::{
    GitRepositoryInfo, GitRepositoryPort,
};

/// Inspecciona repositorios usando la CLI de git.
#[derive(Default)]
pub struct GitCliRepositoryAdapter;

impl GitCliRepositoryAdapter {
    pub fn new() -> Self {
        Self
    }
}

fn git_error(error: GitCommandError) -> WorkspaceError {
    WorkspaceError::Git(error.to_string())
}

#[async_trait::async_trait]
impl GitRepositoryPort for GitCliRepositoryAdapter {
    async fn inspect(&self, path: &Path) -> Result<GitRepositoryInfo, WorkspaceError> {
        if !path.is_dir() {
            return Err(WorkspaceError::PathNotFound(path.display().to_string()));
        }
        let root: PathBuf = match run_git(path, &["rev-parse", "--show-toplevel"]).await {
            Ok(root) => PathBuf::from(root),
            Err(GitCommandError::Failed { .. }) => {
                return Err(WorkspaceError::NotAGitRepository(
                    path.display().to_string(),
                ));
            }
            Err(other) => return Err(git_error(other)),
        };

        let has_commits: bool = run_git_raw(&root, &["rev-parse", "--verify", "--quiet", "HEAD"])
            .await
            .map_err(git_error)?
            .success;
        let branch: GitOutput = run_git_raw(&root, &["symbolic-ref", "--quiet", "--short", "HEAD"])
            .await
            .map_err(git_error)?;
        let current_branch: Option<String> = if has_commits && branch.success {
            Some(branch.stdout.trim().to_string())
        } else {
            None
        };

        Ok(GitRepositoryInfo {
            root,
            current_branch,
        })
    }

    async fn branch_exists(&self, root: &Path, branch: &str) -> Result<bool, WorkspaceError> {
        let reference: String = format!("refs/heads/{branch}");
        let output: GitOutput =
            run_git_raw(root, &["rev-parse", "--verify", "--quiet", &reference])
                .await
                .map_err(git_error)?;
        Ok(output.success)
    }
}
