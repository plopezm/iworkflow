#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use iworkflow::modules::tasks::domain::errors::task_error::TaskError;
use iworkflow::modules::tasks::domain::models::worktree::{DiffRange, MergeOutcome, TaskDiff};
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
async fn diff_live_includes_commits_uncommitted_and_untracked_changes() {
    let f: Fixture = fixture_with_worktree().await;
    std::fs::write(f.worktree.join("committed.txt"), "uno\n").expect("write");
    f.adapter
        .commit_all(&f.worktree, "pi: committed")
        .await
        .expect("commit");
    std::fs::write(f.worktree.join("README.md"), "editado sin commit\n").expect("write");
    std::fs::write(f.worktree.join("untracked.txt"), "nuevo\n").expect("write");

    let diff: TaskDiff = f
        .adapter
        .diff_live(&f.worktree, "main")
        .await
        .expect("live");

    assert!(diff.patch.contains("+uno"));
    assert!(diff.patch.contains("+editado sin commit"));
    assert!(diff.patch.contains("+nuevo"));
    assert!(diff.patch.contains("b/untracked.txt"));
    assert!(diff.stat.contains("untracked.txt"));
    assert!(diff.stat.contains("README.md"));
    assert_eq!(diff.commits.len(), 1);
    // The agent's index is left alone: the new file is still untracked.
    assert!(common::git(&f.worktree, &["status", "--porcelain"]).contains("?? untracked.txt"));
}

#[tokio::test]
async fn diff_live_is_empty_for_a_clean_worktree() {
    let f: Fixture = fixture_with_worktree().await;

    let diff: TaskDiff = f
        .adapter
        .diff_live(&f.worktree, "main")
        .await
        .expect("live");

    assert!(diff.patch.is_empty());
    assert!(diff.commits.is_empty());
}

#[tokio::test]
async fn diff_fingerprint_changes_only_with_commits() {
    let f: Fixture = fixture_with_worktree().await;
    let initial: String = f
        .adapter
        .diff_fingerprint(&f.repo, "main", BRANCH)
        .await
        .expect("fingerprint");
    std::fs::write(f.worktree.join("nuevo.txt"), "hola\n").expect("write");
    let uncommitted: String = f
        .adapter
        .diff_fingerprint(&f.repo, "main", BRANCH)
        .await
        .expect("fingerprint");
    f.adapter
        .commit_all(&f.worktree, "pi: nuevo")
        .await
        .expect("commit");
    let committed: String = f
        .adapter
        .diff_fingerprint(&f.repo, "main", BRANCH)
        .await
        .expect("fingerprint");
    let missing: Result<String, TaskError> = f
        .adapter
        .diff_fingerprint(&f.repo, "main", "no-existe")
        .await;

    // El diff (`base...rama`) ignora lo no commiteado, por eso la huella tampoco lo ve.
    assert_eq!(initial, uncommitted);
    assert_ne!(initial, committed);
    assert!(missing.is_err());
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
async fn remove_deletes_unregistered_folder() {
    let f: Fixture = fixture_with_worktree().await;
    let stray: PathBuf = f._worktrees_dir.path().join("stray");
    std::fs::create_dir_all(&stray).expect("mkdir");
    std::fs::write(stray.join("x.txt"), "x").expect("write");

    f.adapter
        .remove(&f.repo, &stray, "pi/inexistente")
        .await
        .expect("remove");

    assert!(!stray.exists());
}

#[tokio::test]
async fn remove_propagates_failure_for_locked_worktree() {
    let f: Fixture = fixture_with_worktree().await;
    common::git(
        &f.repo,
        &["worktree", "lock", f.worktree.to_str().expect("utf8")],
    );

    let result: Result<(), TaskError> = f.adapter.remove(&f.repo, &f.worktree, BRANCH).await;

    assert!(result.is_err());
    assert!(f.worktree.exists());
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

const PIN_REF: &str = "refs/iworkflow/tasks/1234";

async fn commit_file(f: &Fixture, name: &str, content: &str, message: &str) {
    std::fs::write(f.worktree.join(name), content).expect("write");
    f.adapter
        .commit_all(&f.worktree, message)
        .await
        .expect("commit");
}

#[tokio::test]
async fn pin_range_returns_merge_base_and_tip_and_creates_ref() {
    let f: Fixture = fixture_with_worktree().await;
    let main_sha: String = common::git(&f.repo, &["rev-parse", "main"]);
    commit_file(&f, "nuevo.txt", "hola\n", "pi: nuevo").await;

    let range: DiffRange = f
        .adapter
        .pin_range(&f.repo, "main", BRANCH, PIN_REF)
        .await
        .expect("pin");

    assert_eq!(range.base_sha, main_sha);
    assert_eq!(
        range.head_sha,
        common::git(&f.worktree, &["rev-parse", "HEAD"])
    );
    assert_eq!(
        common::git(&f.repo, &["rev-parse", PIN_REF]),
        range.head_sha
    );
}

#[tokio::test]
async fn diff_range_matches_diff_after_merge_and_cleanup() {
    let f: Fixture = fixture_with_worktree().await;
    commit_file(&f, "nuevo.txt", "hola\n", "pi: nuevo").await;
    commit_file(&f, "otro.txt", "adios\n", "pi: otro").await;
    let before: TaskDiff = f.adapter.diff(&f.repo, "main", BRANCH).await.expect("diff");
    let range: DiffRange = f
        .adapter
        .pin_range(&f.repo, "main", BRANCH, PIN_REF)
        .await
        .expect("pin");

    let outcome: MergeOutcome = f
        .adapter
        .merge(&f.repo, "main", BRANCH, "Merge tarea")
        .await
        .expect("merge");
    f.adapter
        .remove(&f.repo, &f.worktree, BRANCH)
        .await
        .expect("remove");
    let after: TaskDiff = f.adapter.diff_range(&f.repo, &range).await.expect("range");

    assert_eq!(outcome, MergeOutcome::Merged);
    assert_eq!(common::git(&f.repo, &["branch", "--list", BRANCH]), "");
    assert_eq!(after.patch, before.patch);
    assert_eq!(after.stat, before.stat);
    assert_eq!(after.commits, before.commits);
    assert_eq!(after.commits.len(), 2);
    assert_eq!(
        common::git(&f.repo, &["rev-parse", PIN_REF]),
        range.head_sha
    );
}

#[tokio::test]
async fn diff_range_truncates_long_patches_like_diff() {
    let f: Fixture = fixture_with_worktree().await;
    let big: String = "linea de relleno\n".repeat(80_000);
    commit_file(&f, "grande.txt", &big, "pi: grande").await;
    let range: DiffRange = f
        .adapter
        .pin_range(&f.repo, "main", BRANCH, PIN_REF)
        .await
        .expect("pin");

    let plain: TaskDiff = f.adapter.diff(&f.repo, "main", BRANCH).await.expect("diff");
    let ranged: TaskDiff = f.adapter.diff_range(&f.repo, &range).await.expect("range");

    assert!(ranged.truncated);
    assert_eq!(ranged.patch.len(), plain.patch.len());
    assert_eq!(ranged.patch, plain.patch);
}

#[tokio::test]
async fn remove_deletes_folder_that_is_no_longer_a_worktree() {
    let f: Fixture = fixture_with_worktree().await;
    // Simula un worktree corrupto: la carpeta existe pero git ya no la reconoce.
    std::fs::remove_file(f.worktree.join(".git")).expect("remove .git file");

    f.adapter
        .remove(&f.repo, &f.worktree, BRANCH)
        .await
        .expect("remove");

    assert!(!f.worktree.exists());
    assert_eq!(common::git(&f.repo, &["branch", "--list", BRANCH]), "");
    assert!(
        !common::git(&f.repo, &["worktree", "list", "--porcelain"])
            .contains(f.worktree.to_string_lossy().as_ref())
    );
}

#[tokio::test]
async fn remove_refuses_to_delete_folders_outside_the_managed_root() {
    let f: Fixture = fixture_with_worktree().await;
    let outside: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(outside.path().join("x.txt"), "x").expect("write");

    let result: Result<(), TaskError> = f
        .adapter
        .remove(&f.repo, outside.path(), "pi/inexistente")
        .await;

    assert!(matches!(result, Err(TaskError::UnsafeWorktreePath(_))));
    assert!(outside.path().join("x.txt").exists());
}

#[tokio::test]
async fn remove_refuses_to_delete_the_managed_root_itself() {
    let f: Fixture = fixture_with_worktree().await;
    let root: PathBuf = f._worktrees_dir.path().to_path_buf();

    let result: Result<(), TaskError> = f.adapter.remove(&f.repo, &root, "pi/inexistente").await;

    assert!(matches!(result, Err(TaskError::UnsafeWorktreePath(_))));
    assert!(f.worktree.exists());
}

#[cfg(unix)]
#[tokio::test]
async fn remove_refuses_symlinks_that_escape_the_managed_root() {
    let f: Fixture = fixture_with_worktree().await;
    let outside: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(outside.path().join("x.txt"), "x").expect("write");
    let link: PathBuf = f._worktrees_dir.path().join("escape");
    std::os::unix::fs::symlink(outside.path(), &link).expect("symlink");

    let result: Result<(), TaskError> = f.adapter.remove(&f.repo, &link, "pi/inexistente").await;

    assert!(matches!(result, Err(TaskError::UnsafeWorktreePath(_))));
    assert!(outside.path().join("x.txt").exists());
}

#[tokio::test]
async fn remove_parks_worktree_and_purges_it_in_background() {
    let f: Fixture = fixture_with_worktree().await;
    let heavy: PathBuf = f.worktree.join("target").join("debug");
    std::fs::create_dir_all(&heavy).expect("mkdir");
    std::fs::write(heavy.join("big.bin"), "x").expect("write");

    f.adapter
        .remove(&f.repo, &f.worktree, BRANCH)
        .await
        .expect("remove");

    // La carpeta y el registro desaparecen al devolver; el borrado pesado va en segundo plano.
    assert!(!f.worktree.exists());
    assert!(!common::git(&f.repo, &["worktree", "list", "--porcelain"]).contains("1234-feature"));
    assert_eq!(common::git(&f.repo, &["branch", "--list", BRANCH]), "");

    f.adapter.wait_for_cleanup().await;
    let trash: PathBuf = f._worktrees_dir.path().join(".trash");
    assert_eq!(std::fs::read_dir(&trash).expect("trash").count(), 0);
}

/// Repositorio con un remoto `origin` (bare) que tiene `main` y `develop`; `develop` añade
/// `develop.txt`. En el clon solo existe la local `main`; `origin/develop` está sin checkout.
struct RemoteFixture {
    _remote_dir: tempfile::TempDir,
    _repo_dir: tempfile::TempDir,
    _worktrees_dir: tempfile::TempDir,
    repo: PathBuf,
    adapter: GitCliWorktreeAdapter,
}

fn fixture_with_remote_develop() -> RemoteFixture {
    let (repo_dir, repo) = common::git_repo();
    let remote_dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let remote: String = remote_dir.path().to_string_lossy().into_owned();
    common::git(&repo, &["init", "-q", "--bare", &remote]);
    common::git(&repo, &["remote", "add", "origin", &remote]);
    common::git(&repo, &["checkout", "-q", "-b", "develop"]);
    std::fs::write(repo.join("develop.txt"), "dev\n").expect("write");
    common::git(&repo, &["add", "."]);
    common::git(&repo, &["commit", "-q", "-m", "develop"]);
    common::git(&repo, &["push", "-q", "origin", "main", "develop"]);
    common::git(&repo, &["checkout", "-q", "main"]);
    common::git(&repo, &["branch", "-q", "-D", "develop"]);
    let worktrees_dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GitCliWorktreeAdapter =
        GitCliWorktreeAdapter::new(worktrees_dir.path().to_path_buf());
    RemoteFixture {
        _remote_dir: remote_dir,
        _repo_dir: repo_dir,
        _worktrees_dir: worktrees_dir,
        repo,
        adapter,
    }
}

#[tokio::test]
async fn lists_local_and_remote_branches_without_task_or_head_refs() {
    let f: RemoteFixture = fixture_with_remote_develop();
    common::git(&f.repo, &["branch", "pi/otra-tarea-1"]);
    common::git(&f.repo, &["remote", "set-head", "origin", "main"]);

    let names: Vec<(String, bool)> = f
        .adapter
        .list_branches(&f.repo)
        .await
        .expect("branches")
        .into_iter()
        .map(|b| (b.name, b.remote))
        .collect();
    let current: Vec<String> = f
        .adapter
        .list_branches(&f.repo)
        .await
        .expect("branches")
        .into_iter()
        .filter(|b| b.current)
        .map(|b| b.name)
        .collect();

    assert_eq!(current, vec!["main".to_string()]);
    assert_eq!(
        names,
        vec![
            ("main".to_string(), false),
            ("origin/develop".to_string(), true),
            ("origin/main".to_string(), true),
        ]
    );
}

#[tokio::test]
async fn creates_worktree_from_a_remote_branch() {
    let f: RemoteFixture = fixture_with_remote_develop();
    let worktree: PathBuf = f.adapter.worktree_path("demo", "1234-remote");

    f.adapter
        .create(&f.repo, "origin/develop", BRANCH, &worktree)
        .await
        .expect("create");

    assert!(worktree.join("develop.txt").exists());
    assert_eq!(
        common::git(&worktree, &["branch", "--show-current"]),
        BRANCH
    );
}

#[tokio::test]
async fn merges_a_task_based_on_a_remote_branch_into_a_local_tracking_branch() {
    let f: RemoteFixture = fixture_with_remote_develop();
    let worktree: PathBuf = f.adapter.worktree_path("demo", "1234-remote");
    f.adapter
        .create(&f.repo, "origin/develop", BRANCH, &worktree)
        .await
        .expect("create");
    std::fs::write(worktree.join("nuevo.txt"), "hola\n").expect("write");
    f.adapter
        .commit_all(&worktree, "pi: nuevo")
        .await
        .expect("commit");
    let remote_tip: String = common::git(&f.repo, &["rev-parse", "origin/develop"]);

    let outcome: MergeOutcome = f
        .adapter
        .merge(&f.repo, "origin/develop", BRANCH, "Merge tarea")
        .await
        .expect("merge");

    assert!(matches!(outcome, MergeOutcome::Merged));
    assert_eq!(common::git(&f.repo, &["show", "develop:nuevo.txt"]), "hola");
    assert_eq!(
        common::git(&f.repo, &["rev-parse", "origin/develop"]),
        remote_tip,
        "el merge nunca toca la rama remota"
    );
    assert_eq!(
        common::git(&f.repo, &["config", "branch.develop.remote"]),
        "origin"
    );
    assert_eq!(common::git(&f.repo, &["branch", "--show-current"]), "main");
}

fn leftover_dir(adapter: &GitCliWorktreeAdapter, workspace: &str, name: &str) -> PathBuf {
    let dir: PathBuf = adapter.worktree_path(workspace, name);
    std::fs::create_dir_all(dir.join("target").join("debug")).expect("mkdir");
    std::fs::write(dir.join("target").join("debug").join("big.bin"), "x").expect("write");
    dir
}

#[tokio::test]
async fn sweep_orphans_removes_unowned_folders_without_git_metadata() {
    let f: Fixture = fixture_with_worktree().await;
    let orphan: PathBuf = leftover_dir(&f.adapter, "demo", "gone-task");
    let owned: PathBuf = leftover_dir(&f.adapter, "demo", "backlog-task");

    let removed: usize = f
        .adapter
        .sweep_orphans(std::slice::from_ref(&owned))
        .await
        .expect("sweep");

    assert_eq!(removed, 1);
    assert!(!orphan.exists());
    assert!(owned.exists(), "a folder owned by an open task is kept");
    assert!(f.worktree.exists(), "a live worktree is never swept");
}

#[tokio::test]
async fn sweep_orphans_keeps_unowned_live_worktrees() {
    let f: Fixture = fixture_with_worktree().await;

    let removed: usize = f.adapter.sweep_orphans(&[]).await.expect("sweep");

    assert_eq!(removed, 0);
    assert!(f.worktree.join(".git").exists());
}

#[tokio::test]
async fn sweep_orphans_purges_the_trash_and_leaves_hidden_dirs_alone() {
    let f: Fixture = fixture_with_worktree().await;
    let root: &std::path::Path = f._worktrees_dir.path();
    let parked: PathBuf = root.join(".trash").join("half-deleted").join("target");
    std::fs::create_dir_all(&parked).expect("mkdir");
    std::fs::write(parked.join("x.bin"), "x").expect("write");
    let merge_dir: PathBuf = root.join(".merge").join("pending");
    std::fs::create_dir_all(&merge_dir).expect("mkdir");

    f.adapter.sweep_orphans(&[]).await.expect("sweep");

    assert_eq!(
        std::fs::read_dir(root.join(".trash"))
            .expect("trash")
            .count(),
        0
    );
    assert!(merge_dir.exists());
}

#[tokio::test]
async fn sweep_orphans_tolerates_a_missing_root() {
    let adapter: GitCliWorktreeAdapter =
        GitCliWorktreeAdapter::new(PathBuf::from("/nonexistent/iwf-worktrees-root"));

    assert_eq!(adapter.sweep_orphans(&[]).await.expect("sweep"), 0);
}
