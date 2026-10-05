#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use iworkflow::modules::tasks::domain::errors::task_error::TaskError;
use iworkflow::modules::tasks::domain::models::worktree::{MergeOutcome, TaskDiff};
use iworkflow::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;
use iworkflow::modules::tasks::infrastructure::adapters::outbound::git::git_cli_worktree_adapter::GitCliWorktreeAdapter;

const BRANCH: &str = "pi/feature-1234";

struct Fixture {
    _repo_dir: tempfile::TempDir,
    _worktrees_dir: tempfile::TempDir,
    repo: PathBuf,
    worktree: PathBuf,
    adapter: GitCliWorktreeAdapter,
}

async fn fixture_with_worktree() -> Fixture {
    let (repo_dir, repo) = common::git_repo();
    let worktrees_dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GitCliWorktreeAdapter =
        GitCliWorktreeAdapter::new(worktrees_dir.path().to_path_buf());
    let worktree: PathBuf = adapter.worktree_path("demo", "1234-feature");
    adapter
        .create(&repo, "main", BRANCH, &worktree)
        .await
        .expect("create");
    Fixture {
        _repo_dir: repo_dir,
        _worktrees_dir: worktrees_dir,
        repo,
        worktree,
        adapter,
    }
}

#[tokio::test]
async fn creates_worktree_on_new_branch() {
    let f: Fixture = fixture_with_worktree().await;

    assert!(f.worktree.join("README.md").exists());
    assert_eq!(
        common::git(&f.worktree, &["branch", "--show-current"]),
        BRANCH
    );
}

#[tokio::test]
async fn commit_all_reports_whether_there_were_changes() {
    let f: Fixture = fixture_with_worktree().await;

    let nothing: bool = f
        .adapter
        .commit_all(&f.worktree, "vacío")
        .await
        .expect("commit");
    std::fs::write(f.worktree.join("nuevo.txt"), "x\n").expect("write");
    let something: bool = f
        .adapter
        .commit_all(&f.worktree, "pi: nuevo")
        .await
        .expect("commit");

    assert!(!nothing);
    assert!(something);
    assert_eq!(
        common::git(&f.worktree, &["log", "-1", "--format=%s"]),
        "pi: nuevo"
    );
}

#[tokio::test]
async fn diff_lists_changes_and_commits() {
    let f: Fixture = fixture_with_worktree().await;
    std::fs::write(f.worktree.join("nuevo.txt"), "hola\n").expect("write");
    f.adapter
        .commit_all(&f.worktree, "pi: nuevo")
        .await
        .expect("commit");

    let diff: TaskDiff = f.adapter.diff(&f.repo, "main", BRANCH).await.expect("diff");

    assert!(diff.stat.contains("nuevo.txt"));
    assert!(diff.patch.contains("+hola"));
    assert_eq!(diff.commits.len(), 1);
    assert!(!diff.truncated);
}

#[tokio::test]
async fn merges_into_checked_out_base_and_removes_worktree() {
    let f: Fixture = fixture_with_worktree().await;
    std::fs::write(f.worktree.join("nuevo.txt"), "hola\n").expect("write");
    f.adapter
        .commit_all(&f.worktree, "pi: nuevo")
        .await
        .expect("commit");

    let outcome: MergeOutcome = f
        .adapter
        .merge(&f.repo, "main", BRANCH, "Merge tarea")
        .await
        .expect("merge");
    f.adapter
        .remove(&f.repo, &f.worktree, BRANCH)
        .await
        .expect("remove");

    assert_eq!(outcome, MergeOutcome::Merged);
    assert!(f.repo.join("nuevo.txt").exists());
    assert_eq!(
        common::git(&f.repo, &["log", "-1", "--format=%s"]),
        "Merge tarea"
    );
    assert!(!f.worktree.exists());
    assert_eq!(common::git(&f.repo, &["branch", "--list", BRANCH]), "");
}

#[tokio::test]
async fn merges_through_temporary_worktree_when_base_not_checked_out() {
    let f: Fixture = fixture_with_worktree().await;
    common::git(&f.repo, &["checkout", "-q", "-b", "otra"]);
    std::fs::write(f.worktree.join("nuevo.txt"), "hola\n").expect("write");
    f.adapter
        .commit_all(&f.worktree, "pi: nuevo")
        .await
        .expect("commit");

    let outcome: MergeOutcome = f
        .adapter
        .merge(&f.repo, "main", BRANCH, "Merge tarea")
        .await
        .expect("merge");

    assert_eq!(outcome, MergeOutcome::Merged);
    assert_eq!(
        common::git(&f.repo, &["log", "-1", "--format=%s", "main"]),
        "Merge tarea"
    );
    assert_eq!(common::git(&f.repo, &["branch", "--show-current"]), "otra");
    let worktrees: String = common::git(&f.repo, &["worktree", "list"]);
    assert!(!worktrees.contains(".merge"));
}

#[tokio::test]
async fn conflict_is_reported_and_base_left_clean() {
    let f: Fixture = fixture_with_worktree().await;
    std::fs::write(f.worktree.join("README.md"), "desde la tarea\n").expect("write");
    f.adapter
        .commit_all(&f.worktree, "pi: cambia")
        .await
        .expect("commit");
    std::fs::write(f.repo.join("README.md"), "desde main\n").expect("write");
    common::git(&f.repo, &["commit", "-qam", "main cambia"]);

    let outcome: MergeOutcome = f
        .adapter
        .merge(&f.repo, "main", BRANCH, "Merge tarea")
        .await
        .expect("merge");

    assert_eq!(
        outcome,
        MergeOutcome::Conflict {
            files: vec!["README.md".to_string()]
        }
    );
    assert_eq!(common::git(&f.repo, &["status", "--porcelain"]), "");
    assert!(f.worktree.exists());
}

#[tokio::test]
async fn dirty_base_blocks_merge() {
    let f: Fixture = fixture_with_worktree().await;
    std::fs::write(f.repo.join("README.md"), "sin commitear\n").expect("write");

    let result: Result<MergeOutcome, TaskError> = f
        .adapter
        .merge(&f.repo, "main", BRANCH, "Merge tarea")
        .await;

    assert!(matches!(result, Err(TaskError::MergeBlocked(_))));
}

#[tokio::test]
async fn remove_tolerates_missing_worktree_folder() {
    let f: Fixture = fixture_with_worktree().await;
    std::fs::remove_dir_all(&f.worktree).expect("rm");

    f.adapter
        .remove(&f.repo, &f.worktree, BRANCH)
        .await
        .expect("remove");

    assert_eq!(common::git(&f.repo, &["branch", "--list", BRANCH]), "");
}

#[tokio::test]
async fn concurrent_merges_into_the_same_base_both_succeed() {
    let f: Fixture = fixture_with_worktree().await;
    let second_branch: &str = "pi/feature-5678";
    let second: PathBuf = f.adapter.worktree_path("demo", "5678-feature");
    f.adapter
        .create(&f.repo, "main", second_branch, &second)
        .await
        .expect("create second");
    std::fs::write(f.worktree.join("a.txt"), "a\n").expect("write a");
    std::fs::write(second.join("b.txt"), "b\n").expect("write b");
    f.adapter
        .commit_all(&f.worktree, "a")
        .await
        .expect("commit a");
    f.adapter.commit_all(&second, "b").await.expect("commit b");

    let (first_result, second_result) = tokio::join!(
        f.adapter.merge(&f.repo, "main", BRANCH, "merge a"),
        f.adapter.merge(&f.repo, "main", second_branch, "merge b"),
    );

    assert_eq!(first_result.expect("merge a"), MergeOutcome::Merged);
    assert_eq!(second_result.expect("merge b"), MergeOutcome::Merged);
    assert!(f.repo.join("a.txt").exists());
    assert!(f.repo.join("b.txt").exists());
}

/// Provoca un conflicto en README.md entre `main` y la rama de la tarea.
async fn make_conflict(f: &Fixture) {
    std::fs::write(f.worktree.join("README.md"), "desde la tarea\n").expect("write");
    f.adapter
        .commit_all(&f.worktree, "pi: cambia")
        .await
        .expect("commit");
    std::fs::write(f.repo.join("README.md"), "desde main\n").expect("write");
    common::git(&f.repo, &["commit", "-qam", "main cambia"]);
}

fn has_merge_head(repo: &std::path::Path) -> bool {
    std::process::Command::new("git")
        .arg("-C")
        .arg(repo)
        .args(["rev-parse", "-q", "--verify", "MERGE_HEAD"])
        .output()
        .expect("git")
        .status
        .success()
}

#[tokio::test]
async fn conflict_in_checked_out_base_leaves_no_merge_pending() {
    let f: Fixture = fixture_with_worktree().await;
    make_conflict(&f).await;

    let outcome: MergeOutcome = f
        .adapter
        .merge(&f.repo, "main", BRANCH, "Merge tarea")
        .await
        .expect("merge");

    assert!(matches!(outcome, MergeOutcome::Conflict { .. }));
    assert!(!has_merge_head(&f.repo));
    assert_eq!(common::git(&f.repo, &["status", "--porcelain"]), "");
}

#[tokio::test]
async fn conflict_through_temporary_worktree_leaves_nothing_behind() {
    let f: Fixture = fixture_with_worktree().await;
    make_conflict(&f).await;
    common::git(&f.repo, &["checkout", "-q", "-b", "otra"]);
    let before: String = common::git(&f.repo, &["rev-parse", "main"]);

    let outcome: MergeOutcome = f
        .adapter
        .merge(&f.repo, "main", BRANCH, "Merge tarea")
        .await
        .expect("merge");

    assert_eq!(
        outcome,
        MergeOutcome::Conflict {
            files: vec!["README.md".to_string()]
        }
    );
    assert_eq!(common::git(&f.repo, &["rev-parse", "main"]), before);
    let worktrees: String = common::git(&f.repo, &["worktree", "list"]);
    assert!(!worktrees.contains(".merge"));
    let merge_dir: PathBuf = f._worktrees_dir.path().join(".merge");
    let leftovers: usize = std::fs::read_dir(&merge_dir)
        .map(|entries| entries.count())
        .unwrap_or(0);
    assert_eq!(leftovers, 0);
}

#[tokio::test]
async fn stale_merge_worktree_does_not_block_later_merges() {
    let f: Fixture = fixture_with_worktree().await;
    common::git(&f.repo, &["checkout", "-q", "-b", "otra"]);
    std::fs::write(f.worktree.join("nuevo.txt"), "hola\n").expect("write");
    f.adapter
        .commit_all(&f.worktree, "pi: nuevo")
        .await
        .expect("commit");
    // Restos de un merge anterior interrumpido: worktree registrado en la ruta fija antigua.
    let stale: PathBuf = f._worktrees_dir.path().join(".merge").join(
        iworkflow::modules::tasks::domain::models::task::slugify(BRANCH),
    );
    std::fs::create_dir_all(stale.parent().expect("parent")).expect("mkdir");
    common::git(
        &f.repo,
        &[
            "worktree",
            "add",
            "--detach",
            &stale.to_string_lossy(),
            "main",
        ],
    );

    let outcome: MergeOutcome = f
        .adapter
        .merge(&f.repo, "main", BRANCH, "Merge tarea")
        .await
        .expect("merge");

    assert_eq!(outcome, MergeOutcome::Merged);
    assert_eq!(
        common::git(&f.repo, &["log", "-1", "--format=%s", "main"]),
        "Merge tarea"
    );
}

#[tokio::test]
async fn exists_reflects_worktree_folder_presence() {
    let f: Fixture = fixture_with_worktree().await;
    let before: bool = f.adapter.exists(&f.worktree).await;

    f.adapter
        .remove(&f.repo, &f.worktree, BRANCH)
        .await
        .expect("remove");
    let after: bool = f.adapter.exists(&f.worktree).await;

    assert!(before);
    assert!(!after);
}
