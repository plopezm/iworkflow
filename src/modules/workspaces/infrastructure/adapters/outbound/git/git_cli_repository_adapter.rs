use std::path::{Path, PathBuf};

use crate::core::git::{GitCommandError, GitOutput, run_git, run_git_raw, run_git_with_stdin};
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
        let current_branch: Option<String> = if branch.success {
            Some(branch.stdout.trim().to_string())
        } else {
            None
        };

        Ok(GitRepositoryInfo {
            root,
            current_branch,
            has_commits,
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

    async fn init_repository(
        &self,
        path: &Path,
        initial_branch: &str,
    ) -> Result<(), WorkspaceError> {
        tokio::fs::create_dir_all(path)
            .await
            .map_err(|error| WorkspaceError::Io(error.to_string()))?;
        run_git(path, &["init", "-q", "-b", initial_branch])
            .await
            .map_err(git_error)?;
        Ok(())
    }

    async fn create_initial_commit(&self, root: &Path) -> Result<(), WorkspaceError> {
        let empty_tree: String =
            run_git_with_stdin(root, &["hash-object", "-t", "tree", "-w", "--stdin"], &[])
                .await
                .map_err(git_error)?;

        let mut commit_args: Vec<String> = Vec::new();
        if !has_git_identity(root).await? {
            commit_args.push("-c".to_string());
            commit_args.push("user.name=pi-workflow".to_string());
            commit_args.push("-c".to_string());
            commit_args.push("user.email=pi-workflow@localhost".to_string());
        }
        commit_args.push("commit-tree".to_string());
        commit_args.push(empty_tree);
        commit_args.push("-m".to_string());
        commit_args.push("Initial commit".to_string());
        let commit_args_refs: Vec<&str> = commit_args.iter().map(String::as_str).collect();

        let commit_sha: String = run_git(root, &commit_args_refs).await.map_err(git_error)?;
        run_git(root, &["update-ref", "HEAD", &commit_sha])
            .await
            .map_err(git_error)?;
        Ok(())
    }
}

async fn has_git_identity(dir: &Path) -> Result<bool, WorkspaceError> {
    let name: GitOutput = run_git_raw(dir, &["config", "user.name"])
        .await
        .map_err(git_error)?;
    let email: GitOutput = run_git_raw(dir, &["config", "user.email"])
        .await
        .map_err(git_error)?;
    Ok(name.success
        && !name.stdout.trim().is_empty()
        && email.success
        && !email.stdout.trim().is_empty())
}
