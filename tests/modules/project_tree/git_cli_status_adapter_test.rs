use std::path::Path;
use std::process::Command;

use iworkflow::modules::project_tree::domain::models::git_change::{GitChange, GitChangeKind};
use iworkflow::modules::project_tree::domain::ports::outbound::git_status_port::GitStatusPort;
use iworkflow::modules::project_tree::infrastructure::adapters::outbound::git::git_cli_status_adapter::GitCliStatusAdapter;
use tempfile::TempDir;

fn git(dir: &Path, args: &[&str]) {
    let status: std::process::ExitStatus = Command::new("git")
        .arg("-C")
        .arg(dir)
        .args(["-c", "user.name=t", "-c", "user.email=t@t"])
        .args(args)
        .status()
        .expect("git");
    assert!(status.success(), "git {args:?}");
}

fn repo() -> TempDir {
    let dir: TempDir = TempDir::new().expect("tempdir");
    git(dir.path(), &["init", "-q"]);
    std::fs::write(dir.path().join("kept.txt"), "one\n").expect("file");
    std::fs::write(dir.path().join("edited.txt"), "a\nb\n").expect("file");
    git(dir.path(), &["add", "."]);
    git(dir.path(), &["commit", "-q", "-m", "init"]);
    dir
}

fn sorted(mut changes: Vec<GitChange>) -> Vec<(String, GitChangeKind)> {
    changes.sort_by(|a: &GitChange, b: &GitChange| a.path.cmp(&b.path));
    changes
        .into_iter()
        .map(|c: GitChange| (c.path, c.kind))
        .collect()
}

#[tokio::test]
async fn reports_modified_added_and_untracked_files() {
    let dir: TempDir = repo();
    std::fs::write(dir.path().join("edited.txt"), "a\nB\n").expect("edit");
    std::fs::write(dir.path().join("staged.txt"), "x\n").expect("file");
    git(dir.path(), &["add", "staged.txt"]);
    std::fs::create_dir_all(dir.path().join("sub")).expect("dir");
    std::fs::write(dir.path().join("sub/new.txt"), "y\n").expect("file");

    let changes: Vec<GitChange> = GitCliStatusAdapter::new()
        .changes(dir.path())
        .await
        .expect("changes");

    assert_eq!(
        sorted(changes),
        vec![
            ("edited.txt".to_string(), GitChangeKind::Modified),
            ("staged.txt".to_string(), GitChangeKind::Added),
            ("sub/new.txt".to_string(), GitChangeKind::Untracked),
        ]
    );
}

#[tokio::test]
async fn paths_are_relative_to_a_subdirectory_root() {
    let dir: TempDir = repo();
    std::fs::create_dir_all(dir.path().join("app")).expect("dir");
    std::fs::write(dir.path().join("app/a.txt"), "x\n").expect("file");
    std::fs::write(dir.path().join("outside.txt"), "x\n").expect("file");

    let changes: Vec<GitChange> = GitCliStatusAdapter::new()
        .changes(&dir.path().join("app"))
        .await
        .expect("changes");

    assert_eq!(
        sorted(changes),
        vec![("a.txt".to_string(), GitChangeKind::Untracked)]
    );
}

#[tokio::test]
async fn not_a_repository_has_no_changes() {
    let dir: TempDir = TempDir::new().expect("tempdir");

    let changes: Vec<GitChange> = GitCliStatusAdapter::new()
        .changes(dir.path())
        .await
        .expect("changes");

    assert!(changes.is_empty());
}

#[tokio::test]
async fn base_content_is_head_version_or_none() {
    let dir: TempDir = repo();
    std::fs::write(dir.path().join("edited.txt"), "changed\n").expect("edit");
    std::fs::write(dir.path().join("fresh.txt"), "new\n").expect("file");
    let adapter: GitCliStatusAdapter = GitCliStatusAdapter::new();

    assert_eq!(
        adapter.base_content(dir.path(), "edited.txt").await,
        Ok(Some("a\nb\n".to_string()))
    );
    assert_eq!(
        adapter.base_content(dir.path(), "fresh.txt").await,
        Ok(None)
    );
    assert!(adapter.base_content(dir.path(), "../x").await.is_err());
}
