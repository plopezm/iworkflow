#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use pi_workflow::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use pi_workflow::modules::workspaces::domain::ports::outbound::git_repository_port::{
    GitRepositoryInfo, GitRepositoryPort,
};
use pi_workflow::modules::workspaces::infrastructure::adapters::outbound::git::git_cli_repository_adapter::GitCliRepositoryAdapter;

#[tokio::test]
async fn inspect_resolves_root_and_branch_from_subfolder() {
    let (_dir, root) = common::git_repo();
    let sub: PathBuf = root.join("sub");
    std::fs::create_dir(&sub).expect("mkdir");

    let info: GitRepositoryInfo = GitCliRepositoryAdapter::new()
        .inspect(&sub)
        .await
        .expect("inspect");

    assert_eq!(info.root, root);
    assert_eq!(info.current_branch.as_deref(), Some("main"));
}

#[tokio::test]
async fn inspect_reports_no_branch_for_repository_without_commits() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    common::git(dir.path(), &["init", "-q"]);

    let info: GitRepositoryInfo = GitCliRepositoryAdapter::new()
        .inspect(dir.path())
        .await
        .expect("inspect");

    assert_eq!(info.current_branch, None);
}

#[tokio::test]
async fn inspect_rejects_plain_folder_and_missing_path() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GitCliRepositoryAdapter = GitCliRepositoryAdapter::new();

    let plain: Result<GitRepositoryInfo, WorkspaceError> = adapter.inspect(dir.path()).await;
    let missing: Result<GitRepositoryInfo, WorkspaceError> =
        adapter.inspect(&dir.path().join("nope")).await;

    assert!(matches!(plain, Err(WorkspaceError::NotAGitRepository(_))));
    assert!(matches!(missing, Err(WorkspaceError::PathNotFound(_))));
}

#[tokio::test]
async fn branch_exists_checks_local_branches() {
    let (_dir, root) = common::git_repo();
    common::git(&root, &["branch", "develop"]);
    let adapter: GitCliRepositoryAdapter = GitCliRepositoryAdapter::new();

    assert!(
        adapter
            .branch_exists(&root, "develop")
            .await
            .expect("check")
    );
    assert!(!adapter.branch_exists(&root, "nope").await.expect("check"));
}
