#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use pi_workflow::modules::tasks::domain::errors::task_error::TaskError;
use pi_workflow::modules::tasks::domain::models::worktree::{MergeOutcome, TaskDiff};
use pi_workflow::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;
use pi_workflow::modules::tasks::infrastructure::adapters::outbound::git::git_cli_worktree_adapter::GitCliWorktreeAdapter;

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
