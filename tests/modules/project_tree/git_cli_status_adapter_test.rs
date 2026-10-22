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

fn flags(changes: &[GitChange], path: &str) -> (bool, bool) {
    let change: &GitChange = changes
        .iter()
        .find(|c: &&GitChange| c.path == path)
        .expect(path);
    (change.staged, change.unstaged)
}

fn cached(dir: &Path) -> String {
    let output: std::process::Output = Command::new("git")
        .arg("-C")
        .arg(dir)
        .args(["diff", "--cached", "--name-only"])
        .output()
        .expect("git");
    String::from_utf8_lossy(&output.stdout).trim().to_string()
}

#[tokio::test]
async fn stage_adds_only_the_given_paths_and_unstage_keeps_the_content() {
    let dir: TempDir = repo();
    std::fs::write(dir.path().join("edited.txt"), "a\nB\n").expect("edit");
    std::fs::write(dir.path().join("fresh.txt"), "new\n").expect("file");
    let adapter: GitCliStatusAdapter = GitCliStatusAdapter::new();

    adapter
        .stage(dir.path(), &["edited.txt".to_string()])
        .await
        .expect("stage");
    assert_eq!(cached(dir.path()), "edited.txt");
    let changes: Vec<GitChange> = adapter.changes(dir.path()).await.expect("changes");
    assert_eq!(flags(&changes, "edited.txt"), (true, false));
    assert_eq!(flags(&changes, "fresh.txt"), (false, true));

    adapter
        .unstage(dir.path(), &["edited.txt".to_string()])
        .await
        .expect("unstage");
    assert_eq!(cached(dir.path()), "");
    assert_eq!(
        std::fs::read_to_string(dir.path().join("edited.txt")).expect("read"),
        "a\nB\n"
    );
}

#[tokio::test]
async fn empty_path_list_stages_and_unstages_everything() {
    let dir: TempDir = repo();
    std::fs::write(dir.path().join("edited.txt"), "a\nB\n").expect("edit");
    std::fs::write(dir.path().join("fresh.txt"), "new\n").expect("file");
    let adapter: GitCliStatusAdapter = GitCliStatusAdapter::new();

    adapter.stage(dir.path(), &[]).await.expect("stage all");
    assert_eq!(cached(dir.path()), "edited.txt\nfresh.txt");

    adapter.unstage(dir.path(), &[]).await.expect("unstage all");
    assert_eq!(cached(dir.path()), "");
    assert!(dir.path().join("fresh.txt").exists());
}

#[tokio::test]
async fn partially_staged_file_is_both_staged_and_unstaged() {
    let dir: TempDir = repo();
    let adapter: GitCliStatusAdapter = GitCliStatusAdapter::new();
    std::fs::write(dir.path().join("edited.txt"), "a\nB\n").expect("edit");
    adapter
        .stage(dir.path(), &["edited.txt".to_string()])
        .await
        .expect("stage");
    std::fs::write(dir.path().join("edited.txt"), "a\nB\nC\n").expect("edit again");

    let changes: Vec<GitChange> = adapter.changes(dir.path()).await.expect("changes");

    assert_eq!(flags(&changes, "edited.txt"), (true, true));
}

#[tokio::test]
async fn unstage_works_before_the_first_commit() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    git(dir.path(), &["init", "-q"]);
    std::fs::write(dir.path().join("a.txt"), "x\n").expect("file");
    let adapter: GitCliStatusAdapter = GitCliStatusAdapter::new();
    adapter.stage(dir.path(), &[]).await.expect("stage");

    adapter.unstage(dir.path(), &[]).await.expect("unstage");

    let changes: Vec<GitChange> = adapter.changes(dir.path()).await.expect("changes");
    assert_eq!(flags(&changes, "a.txt"), (false, true));
}

#[tokio::test]
async fn special_and_escaping_paths_are_never_treated_as_pathspec_magic() {
    let dir: TempDir = repo();
    std::fs::write(dir.path().join("edited.txt"), "a\nB\n").expect("edit");
    let adapter: GitCliStatusAdapter = GitCliStatusAdapter::new();

    assert!(
        adapter
            .stage(dir.path(), &["../x".to_string()])
            .await
            .is_err()
    );
    assert!(adapter.stage(dir.path(), &[String::new()]).await.is_err());
    // A glob is a literal file name, so it matches nothing and must not stage `edited.txt`.
    let _ = adapter.stage(dir.path(), &["*.txt".to_string()]).await;
    assert_eq!(cached(dir.path()), "");
}

#[tokio::test]
async fn commit_records_only_the_staged_changes() {
    let dir: TempDir = repo();
    std::fs::write(dir.path().join("edited.txt"), "a\nB\n").expect("edit");
    std::fs::write(dir.path().join("other.txt"), "z\n").expect("file");
    git(dir.path(), &["add", "edited.txt"]);

    GitCliStatusAdapter::new()
        .commit(dir.path(), "feat: edit")
        .await
        .expect("commit");

    let changes: Vec<GitChange> = GitCliStatusAdapter::new()
        .changes(dir.path())
        .await
        .expect("changes");
    assert_eq!(
        sorted(changes),
        vec![("other.txt".to_string(), GitChangeKind::Untracked)]
    );
}

#[tokio::test]
async fn commit_without_staged_changes_fails() {
    let dir: TempDir = repo();
    std::fs::write(dir.path().join("edited.txt"), "a\nB\n").expect("edit");

    let result = GitCliStatusAdapter::new()
        .commit(dir.path(), "nothing")
        .await;

    assert!(result.is_err());
}
