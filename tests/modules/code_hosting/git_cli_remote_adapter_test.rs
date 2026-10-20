#[path = "../../common/mod.rs"]
mod common;

use base64::Engine;
use base64::engine::general_purpose::STANDARD;
use iworkflow::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use iworkflow::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;
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

/// Servidor HTTP mínimo que registra la primera petición completa y responde 401.
async fn recording_server() -> (String, tokio::task::JoinHandle<String>) {
    let listener: TcpListener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
    let url: String = format!("http://{}", listener.local_addr().expect("addr"));
    let handle: tokio::task::JoinHandle<String> = tokio::spawn(async move {
        let (mut socket, _) = listener.accept().await.expect("accept");
        let mut received: Vec<u8> = Vec::new();
        let mut chunk: [u8; 1024] = [0; 1024];
        while !received.windows(4).any(|w: &[u8]| w == b"\r\n\r\n") {
            let read: usize = socket.read(&mut chunk).await.expect("read");
            if read == 0 {
                break;
            }
            received.extend_from_slice(&chunk[..read]);
        }
        let reply: &str = "HTTP/1.1 401 Unauthorized\r\nWWW-Authenticate: Basic realm=\"git\"\r\nContent-Length: 0\r\nConnection: close\r\n\r\n";
        socket.write_all(reply.as_bytes()).await.expect("write");
        String::from_utf8_lossy(&received).into_owned()
    });
    (url, handle)
}

#[tokio::test(flavor = "multi_thread")]
async fn publish_branch_pushes_to_the_provider_url_authenticating_with_the_token() {
    let (_dir, repo) = common::git_repo();
    // `origin` es SSH: el push no debe depender de él ni de las credenciales del sistema.
    common::git(
        &repo,
        &[
            "remote",
            "add",
            "origin",
            "git@unreachable.invalid:acme/app.git",
        ],
    );
    common::git(&repo, &["branch", "pi/feature"]);
    let (url, request) = recording_server().await;
    let settings: CodeHostSettings =
        CodeHostSettings::new(HostingProvider::GitHub, &url, "ghp_secret", false)
            .expect("settings");

    let result: Result<(), CodeHostError> = GitCliRemoteAdapter::new()
        .publish_branch(&repo, "pi/feature", &settings, "acme/app")
        .await;

    let request: String = request.await.expect("request");
    let expected: String = STANDARD.encode("x-access-token:ghp_secret");
    assert!(
        request.starts_with("GET /acme/app.git/info/refs?service=git-receive-pack"),
        "{request}"
    );
    assert!(
        request
            .to_ascii_lowercase()
            .contains(&format!("authorization: basic {expected}").to_ascii_lowercase()),
        "{request}"
    );
    // El servidor falso rechaza con 401: el fallo se informa sin filtrar el token.
    match result {
        Err(CodeHostError::BranchPublishFailed { reason, .. }) => {
            assert!(!reason.contains("ghp_secret"), "{reason}");
        }
        other => panic!("se esperaba BranchPublishFailed, llegó {other:?}"),
    }
}

#[tokio::test(flavor = "multi_thread")]
async fn publish_branch_authenticates_gitlab_with_the_oauth2_user() {
    let (_dir, repo) = common::git_repo();
    common::git(&repo, &["branch", "pi/feature"]);
    let (url, request) = recording_server().await;
    let settings: CodeHostSettings =
        CodeHostSettings::new(HostingProvider::GitLab, &url, "glpat-secret", false)
            .expect("settings");

    let _ = GitCliRemoteAdapter::new()
        .publish_branch(&repo, "pi/feature", &settings, "grupo/proyecto")
        .await;

    let request: String = request.await.expect("request").to_ascii_lowercase();
    let expected: String = STANDARD.encode("oauth2:glpat-secret").to_ascii_lowercase();
    assert!(request.contains(&format!("authorization: basic {expected}")));
}

#[tokio::test(flavor = "multi_thread")]
async fn publish_branch_reports_failure_when_the_server_is_unreachable() {
    let (_dir, repo) = common::git_repo();
    common::git(&repo, &["branch", "pi/feature"]);
    let settings: CodeHostSettings =
        CodeHostSettings::new(HostingProvider::GitLab, "http://127.0.0.1:1", "tok", false)
            .expect("settings");

    let result: Result<(), CodeHostError> = GitCliRemoteAdapter::new()
        .publish_branch(&repo, "pi/feature", &settings, "grupo/proyecto")
        .await;

    assert!(matches!(
        result,
        Err(CodeHostError::BranchPublishFailed { .. })
    ));
}

#[tokio::test(flavor = "multi_thread")]
async fn ensure_commits_ahead_adds_an_empty_commit_when_branch_has_none() {
    let (_dir, repo) = common::git_repo();
    common::git(&repo, &["branch", "pi/feature"]);
    let tree_before: String = common::git(&repo, &["rev-parse", "pi/feature^{tree}"]);

    let created: bool = GitCliRemoteAdapter::new()
        .ensure_commits_ahead(&repo, "pi/feature", "HEAD")
        .await
        .expect("ensure");

    assert!(created);
    assert_eq!(
        common::git(&repo, &["rev-list", "--count", "HEAD..pi/feature"]).trim(),
        "1"
    );
    assert_eq!(
        common::git(&repo, &["rev-parse", "pi/feature^{tree}"]),
        tree_before
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn ensure_commits_ahead_leaves_a_branch_that_already_advances() {
    let (_dir, repo) = common::git_repo();
    common::git(&repo, &["checkout", "-b", "pi/feature"]);
    common::git(&repo, &["commit", "--allow-empty", "-m", "work"]);
    let head_before: String = common::git(&repo, &["rev-parse", "pi/feature"]);

    let created: bool = GitCliRemoteAdapter::new()
        .ensure_commits_ahead(&repo, "pi/feature", "HEAD~1")
        .await
        .expect("ensure");

    assert!(!created);
    assert_eq!(
        common::git(&repo, &["rev-parse", "pi/feature"]),
        head_before
    );
}
