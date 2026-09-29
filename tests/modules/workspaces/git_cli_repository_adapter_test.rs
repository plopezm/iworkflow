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
    assert!(info.has_commits);
}

#[tokio::test]
async fn inspect_reports_symbolic_branch_without_commits() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    common::git(dir.path(), &["init", "-q", "-b", "main"]);

    let info: GitRepositoryInfo = GitCliRepositoryAdapter::new()
        .inspect(dir.path())
        .await
        .expect("inspect");

    assert_eq!(info.current_branch.as_deref(), Some("main"));
    assert!(!info.has_commits);
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

#[tokio::test]
async fn init_repository_creates_nested_missing_dir_on_requested_branch() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let target: PathBuf = dir.path().join("nested").join("repo");
    let adapter: GitCliRepositoryAdapter = GitCliRepositoryAdapter::new();

    adapter
        .init_repository(&target, "develop")
        .await
        .expect("init");

    let info: GitRepositoryInfo = adapter.inspect(&target).await.expect("inspect");
    assert_eq!(info.current_branch.as_deref(), Some("develop"));
    assert!(!info.has_commits);
}

#[tokio::test]
async fn create_initial_commit_leaves_staged_file_uncommitted() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    common::git(dir.path(), &["init", "-q", "-b", "main"]);
    common::git(dir.path(), &["config", "user.email", "test@example.com"]);
    common::git(dir.path(), &["config", "user.name", "Test"]);
    std::fs::write(dir.path().join("staged.txt"), "pending\n").expect("write");
    common::git(dir.path(), &["add", "staged.txt"]);
    let adapter: GitCliRepositoryAdapter = GitCliRepositoryAdapter::new();

    adapter
        .create_initial_commit(dir.path())
        .await
        .expect("commit");

    let info: GitRepositoryInfo = adapter.inspect(dir.path()).await.expect("inspect");
    assert!(info.has_commits);
    assert_eq!(info.current_branch.as_deref(), Some("main"));

    let show: String = common::git(dir.path(), &["show", "--stat", "--format=", "HEAD"]);
    assert!(
        show.trim().is_empty(),
        "el commit inicial no debe incluir archivos: {show}"
    );
    let status: String = common::git(dir.path(), &["status", "--porcelain"]);
    assert!(status.contains("staged.txt"));
}
