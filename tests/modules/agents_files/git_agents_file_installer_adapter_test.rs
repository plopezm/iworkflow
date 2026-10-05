#[path = "../../common/mod.rs"]
mod common;

use std::path::{Path, PathBuf};

use iworkflow::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use iworkflow::modules::agents_files::domain::models::install_outcome::InstallOutcome;
use iworkflow::modules::agents_files::domain::ports::outbound::agents_file_installer_port::AgentsFileInstallerPort;
use iworkflow::modules::agents_files::infrastructure::adapters::outbound::git::git_agents_file_installer_adapter::GitAgentsFileInstallerAdapter;
use tempfile::TempDir;

#[tokio::test]
async fn commits_only_agents_md_and_leaves_other_changes_untouched() {
    let (_guard, root): (TempDir, PathBuf) = common::git_repo();
    std::fs::write(root.join("README.md"), "modificado\n").expect("write");
    std::fs::write(root.join("notes.txt"), "sin seguimiento\n").expect("write");
    let adapter: GitAgentsFileInstallerAdapter = GitAgentsFileInstallerAdapter::new();

    let outcome: InstallOutcome = adapter
        .install(b"# reglas\n", &root, false)
        .await
        .expect("install");

    assert!(outcome.committed);
    assert_eq!(
        std::fs::read_to_string(root.join("AGENTS.md")).expect("read"),
        "# reglas\n"
    );
    assert_eq!(
        common::git(&root, &["log", "-1", "--format=%s"]),
        "docs: add AGENTS.md"
    );
    assert_eq!(
        common::git(&root, &["show", "--name-only", "--format=", "HEAD"]),
        "AGENTS.md"
    );
    let status: String = common::git(&root, &["status", "--porcelain"]);
    assert!(status.contains("M README.md"), "{status}");
    assert!(status.contains("?? notes.txt"), "{status}");
    assert!(!status.contains("AGENTS.md"), "{status}");
}

#[tokio::test]
async fn existing_file_without_overwrite_is_rejected_and_kept() {
    let (_guard, root): (TempDir, PathBuf) = common::git_repo();
    std::fs::write(root.join("AGENTS.md"), "original\n").expect("write");
    let adapter: GitAgentsFileInstallerAdapter = GitAgentsFileInstallerAdapter::new();

    let result: Result<InstallOutcome, AgentsFileError> =
        adapter.install(b"nuevo\n", &root, false).await;

    assert_eq!(
        result,
        Err(AgentsFileError::TargetExists(
            root.join("AGENTS.md").display().to_string()
        ))
    );
    assert_eq!(
        std::fs::read_to_string(root.join("AGENTS.md")).expect("read"),
        "original\n"
    );
}

#[tokio::test]
async fn overwrite_replaces_and_commits_the_new_content() {
    let (_guard, root): (TempDir, PathBuf) = common::git_repo();
    let adapter: GitAgentsFileInstallerAdapter = GitAgentsFileInstallerAdapter::new();
    adapter
        .install(b"original\n", &root, false)
        .await
        .expect("first");

    let outcome: InstallOutcome = adapter
        .install(b"nuevo\n", &root, true)
        .await
        .expect("overwrite");

    assert!(outcome.committed);
    assert_eq!(common::git(&root, &["show", "HEAD:AGENTS.md"]), "nuevo");
    assert_eq!(common::git(&root, &["status", "--porcelain"]), "");
}

#[tokio::test]
async fn overwrite_with_identical_content_is_not_committed_again() {
    let (_guard, root): (TempDir, PathBuf) = common::git_repo();
    let adapter: GitAgentsFileInstallerAdapter = GitAgentsFileInstallerAdapter::new();
    adapter
        .install(b"igual\n", &root, false)
        .await
        .expect("first");
    let before: String = common::git(&root, &["rev-parse", "HEAD"]);

    let outcome: InstallOutcome = adapter
        .install(b"igual\n", &root, true)
        .await
        .expect("overwrite");

    assert!(!outcome.committed);
    assert_eq!(common::git(&root, &["rev-parse", "HEAD"]), before);
}

#[tokio::test]
async fn non_git_directory_writes_without_committing() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GitAgentsFileInstallerAdapter = GitAgentsFileInstallerAdapter::new();

    let outcome: InstallOutcome = adapter
        .install(b"# reglas\n", dir.path(), false)
        .await
        .expect("install");

    assert!(!outcome.committed);
    assert_eq!(
        std::fs::read_to_string(dir.path().join("AGENTS.md")).expect("read"),
        "# reglas\n"
    );
}

#[tokio::test]
async fn missing_directory_is_an_install_error() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let missing: PathBuf = dir.path().join("nope");
    let adapter: GitAgentsFileInstallerAdapter = GitAgentsFileInstallerAdapter::new();

    let result: Result<InstallOutcome, AgentsFileError> =
        adapter.install(b"x", Path::new(&missing), false).await;

    assert!(matches!(result, Err(AgentsFileError::Install(_))));
}
