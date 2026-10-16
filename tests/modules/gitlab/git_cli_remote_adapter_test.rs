#[path = "../../common/mod.rs"]
mod common;

use iworkflow::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use iworkflow::modules::gitlab::domain::ports::outbound::git_remote_port::GitRemotePort;
use iworkflow::modules::gitlab::infrastructure::adapters::outbound::git::git_cli_remote_adapter::GitCliRemoteAdapter;

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

    let result: Result<String, GitLabError> = GitCliRemoteAdapter::new().origin_url(&repo).await;

    assert!(matches!(result, Err(GitLabError::NoOrigin(_))));
}

#[tokio::test(flavor = "multi_thread")]
async fn origin_url_ignores_other_remotes() {
    let (_dir, repo) = common::git_repo();
    common::git(
        &repo,
        &["remote", "add", "upstream", "https://x.corp/a/b.git"],
    );

    let result: Result<String, GitLabError> = GitCliRemoteAdapter::new().origin_url(&repo).await;

    assert!(matches!(result, Err(GitLabError::NoOrigin(_))));
}
