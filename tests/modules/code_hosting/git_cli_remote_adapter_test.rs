#[path = "../../common/mod.rs"]
mod common;

use iworkflow::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use iworkflow::modules::code_hosting::domain::ports::outbound::git_remote_port::GitRemotePort;
use iworkflow::modules::code_hosting::infrastructure::adapters::outbound::git::git_cli_remote_adapter::GitCliRemoteAdapter;

#[tokio::test(flavor = "multi_thread")]
async fn origin_url_returns_configured_remote() {
    let (_dir, repo) = common::git_repo();
    common::git(
        &repo,
        &["remote", "add", "origin", "git@git.corp:grupo/proyecto.git"],
    );

    let url: String = GitCliRemoteAdapter::new()
        .origin_url(&repo)
        .await
        .expect("origin");

    assert_eq!(url, "git@git.corp:grupo/proyecto.git");
}

#[tokio::test(flavor = "multi_thread")]
async fn origin_url_fails_with_no_origin_when_missing() {
    let (_dir, repo) = common::git_repo();

    let result: Result<String, CodeHostError> = GitCliRemoteAdapter::new().origin_url(&repo).await;

    assert!(matches!(result, Err(CodeHostError::NoOrigin(_))));
}

#[tokio::test(flavor = "multi_thread")]
async fn origin_url_ignores_other_remotes() {
    let (_dir, repo) = common::git_repo();
    common::git(
        &repo,
        &["remote", "add", "upstream", "https://x.corp/a/b.git"],
    );

    let result: Result<String, CodeHostError> = GitCliRemoteAdapter::new().origin_url(&repo).await;

    assert!(matches!(result, Err(CodeHostError::NoOrigin(_))));
}

#[tokio::test(flavor = "multi_thread")]
async fn publish_branch_pushes_the_local_branch_to_origin() {
    let (_dir, repo) = common::git_repo();
    let remote_dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    common::git(remote_dir.path(), &["init", "--bare", "-q"]);
    common::git(
        &repo,
        &[
            "remote",
            "add",
            "origin",
            &remote_dir.path().display().to_string(),
        ],
    );
    common::git(&repo, &["branch", "pi/feature"]);

    GitCliRemoteAdapter::new()
        .publish_branch(&repo, "pi/feature")
        .await
        .expect("publish");

    common::git(
        remote_dir.path(),
        &["rev-parse", "--verify", "refs/heads/pi/feature"],
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn publish_branch_reports_failure_when_origin_is_unreachable() {
    let (_dir, repo) = common::git_repo();
    common::git(
        &repo,
        &["remote", "add", "origin", "/nonexistent/remote.git"],
    );
    common::git(&repo, &["branch", "pi/feature"]);

    let result: Result<(), CodeHostError> = GitCliRemoteAdapter::new()
        .publish_branch(&repo, "pi/feature")
        .await;

    assert!(matches!(
        result,
        Err(CodeHostError::BranchPublishFailed { .. })
    ));
}
