use std::net::SocketAddr;
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};

use axum::Router;
use axum::extract::State;
use axum::middleware::from_fn_with_state;
use axum::routing::{get, post};
use iworkflow::modules::preferences::domain::models::access_token::AccessToken;
use iworkflow::modules::preferences::infrastructure::adapters::inbound::access_guard::{
    AccessGuard, access_router, require_access,
};
use reqwest::header::{
    ACCEPT, AUTHORIZATION, CACHE_CONTROL, CONTENT_TYPE, COOKIE, LOCATION, SET_COOKIE,
};
use reqwest::{Client, Response, StatusCode};

struct TestServer {
    base: String,
    hits: Arc<AtomicUsize>,
    client: Client,
}

async fn hello(State(hits): State<Arc<AtomicUsize>>) -> &'static str {
    hits.fetch_add(1, Ordering::SeqCst);
    "hello"
}

async fn spawn(guard: AccessGuard) -> TestServer {
    let hits: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
    let rest: Router = Router::new()
        .route("/hello", get(hello).post(hello))
        .route("/other", post(hello))
        // Like the real app, whose Leptos fallback answers any path nothing else matched.
        .fallback(hello)
        .with_state(hits.clone());
    let app: Router = Router::new()
        .merge(access_router(guard.clone()))
        .merge(rest)
        .layer(from_fn_with_state(guard, require_access));
    let listener: tokio::net::TcpListener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind");
    let addr: SocketAddr = listener.local_addr().expect("addr");
    tokio::spawn(async move {
        axum::serve(listener, app).await.expect("serve");
    });
    TestServer {
        base: format!("http://{addr}"),
        hits,
        client: Client::builder()
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .expect("client"),
    }
}

fn header(response: &Response, name: reqwest::header::HeaderName) -> String {
    response
        .headers()
        .get(name)
        .and_then(|value| value.to_str().ok())
        .unwrap_or_default()
        .to_string()
}

fn cookie_for(token: &AccessToken) -> String {
    format!("iw-token={}", token.as_str())
}

fn expected_set_cookie(token: &AccessToken) -> String {
    format!(
        "iw-token={}; HttpOnly; SameSite=Strict; Path=/; Max-Age=31536000",
        token.as_str()
    )
}

#[tokio::test]
async fn s1_without_enforcement_everything_passes() {
    let server: TestServer = spawn(AccessGuard::new(AccessToken::generate(), false)).await;
    let response: Response = server
        .client
        .get(format!("{}/hello", server.base))
        .send()
        .await
        .expect("send");
    assert_eq!(response.status(), StatusCode::OK);
    assert_eq!(server.hits.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn s2_bearer_cookie_and_cookie_among_others_pass() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token.clone(), true)).await;
    let url: String = format!("{}/hello", server.base);
    let bearer: Response = server
        .client
        .get(&url)
        .header(AUTHORIZATION, format!("Bearer {}", token.as_str()))
        .send()
        .await
        .expect("send");
    assert_eq!(bearer.status(), StatusCode::OK);
    let cookie: Response = server
        .client
        .get(&url)
        .header(COOKIE, format!("a=1; {}; iw-lang=es", cookie_for(&token)))
        .send()
        .await
        .expect("send");
    assert_eq!(cookie.status(), StatusCode::OK);
    assert_eq!(server.hits.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn s3_query_token_redirects_without_the_token_and_sets_cookie() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token.clone(), true)).await;
    let response: Response = server
        .client
        .get(format!(
            "{}/hello?x=1&token={}&y=2",
            server.base,
            token.as_str()
        ))
        .send()
        .await
        .expect("send");
    assert_eq!(response.status(), StatusCode::SEE_OTHER);
    assert_eq!(header(&response, LOCATION), "/hello?x=1&y=2");
    assert_eq!(header(&response, SET_COOKIE), expected_set_cookie(&token));
    assert_eq!(header(&response, CACHE_CONTROL), "no-store");

    let only: Response = server
        .client
        .get(format!("{}/hello?token={}", server.base, token.as_str()))
        .send()
        .await
        .expect("send");
    assert_eq!(only.status(), StatusCode::SEE_OTHER);
    assert_eq!(header(&only, LOCATION), "/hello");
    assert_eq!(server.hits.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn s3_wrong_query_token_is_rejected_and_post_authenticates_without_redirect() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token.clone(), true)).await;
    let wrong: Response = server
        .client
        .get(format!("{}/hello?token={}", server.base, "0".repeat(64)))
        .send()
        .await
        .expect("send");
    assert_eq!(wrong.status(), StatusCode::UNAUTHORIZED);
    assert!(wrong.headers().get(SET_COOKIE).is_none());
    assert_eq!(server.hits.load(Ordering::SeqCst), 0);

    let post: Response = server
        .client
        .post(format!("{}/other?token={}", server.base, token.as_str()))
        .send()
        .await
        .expect("send");
    assert_eq!(post.status(), StatusCode::OK);
    assert_eq!(server.hits.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn s4_rejects_html_with_login_page_and_others_with_empty_body() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token.clone(), true)).await;
    let url: String = format!("{}/hello", server.base);
    let page: Response = server
        .client
        .get(&url)
        .header(ACCEPT, "text/html,application/xhtml+xml")
        .header(COOKIE, "iw-lang=de")
        .send()
        .await
        .expect("send");
    assert_eq!(page.status(), StatusCode::UNAUTHORIZED);
    assert_eq!(header(&page, CONTENT_TYPE), "text/html; charset=utf-8");
    assert_eq!(header(&page, CACHE_CONTROL), "no-store");
    let body: String = page.text().await.expect("body");
    assert!(body.contains("<form method=\"post\" action=\"/access-login\">"));
    assert!(body.contains("name=\"token\""));
    assert!(body.contains("type=\"password\""));
    assert!(body.contains("Zugriffstoken"));
    assert!(!body.contains(token.as_str()));

    let default_lang: String = server
        .client
        .get(&url)
        .header(ACCEPT, "text/html")
        .send()
        .await
        .expect("send")
        .text()
        .await
        .expect("body");
    assert!(default_lang.contains("Access token required"));

    let api: Response = server.client.get(&url).send().await.expect("send");
    assert_eq!(api.status(), StatusCode::UNAUTHORIZED);
    assert_eq!(header(&api, CACHE_CONTROL), "no-store");
    assert!(api.text().await.expect("body").is_empty());

    let post_html: Response = server
        .client
        .post(&url)
        .header(ACCEPT, "text/html")
        .send()
        .await
        .expect("send");
    assert_eq!(post_html.status(), StatusCode::UNAUTHORIZED);
    assert!(post_html.text().await.expect("body").is_empty());
    assert_eq!(server.hits.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn s5_login_form_sets_cookie_or_shows_the_page_again() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token.clone(), true)).await;
    let url: String = format!("{}/access-login", server.base);
    let ok: Response = server
        .client
        .post(&url)
        .form(&[("token", token.as_str())])
        .send()
        .await
        .expect("send");
    assert_eq!(ok.status(), StatusCode::SEE_OTHER);
    assert_eq!(header(&ok, LOCATION), "/");
    assert_eq!(header(&ok, SET_COOKIE), expected_set_cookie(&token));
    assert_eq!(header(&ok, CACHE_CONTROL), "no-store");

    let bad: Response = server
        .client
        .post(&url)
        .header(COOKIE, "iw-lang=fr")
        .form(&[("token", "nope")])
        .send()
        .await
        .expect("send");
    assert_eq!(bad.status(), StatusCode::UNAUTHORIZED);
    assert!(bad.headers().get(SET_COOKIE).is_none());
    assert_eq!(header(&bad, CONTENT_TYPE), "text/html; charset=utf-8");
    let body: String = bad.text().await.expect("body");
    assert!(body.contains("Jeton invalide."));
    assert!(!body.contains("nope"));

    let oversized: Response = server
        .client
        .post(&url)
        .form(&[("token", "a".repeat(10_000))])
        .send()
        .await
        .expect("send");
    assert_eq!(oversized.status(), StatusCode::PAYLOAD_TOO_LARGE);
}

#[tokio::test]
async fn s5_login_route_redirects_home_when_not_enforcing() {
    let server: TestServer = spawn(AccessGuard::new(AccessToken::generate(), false)).await;
    let response: Response = server
        .client
        .post(format!("{}/access-login", server.base))
        .form(&[("token", "whatever")])
        .send()
        .await
        .expect("send");
    assert_eq!(response.status(), StatusCode::SEE_OTHER);
    assert_eq!(header(&response, LOCATION), "/");
    assert!(response.headers().get(SET_COOKIE).is_none());
}

#[tokio::test]
async fn s6_arbitrary_route_needs_the_cookie() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token.clone(), true)).await;
    let url: String = format!("{}/hello", server.base);
    let denied: Response = server.client.get(&url).send().await.expect("send");
    assert_eq!(denied.status(), StatusCode::UNAUTHORIZED);
    assert_eq!(server.hits.load(Ordering::SeqCst), 0);
    let allowed: Response = server
        .client
        .get(&url)
        .header(COOKIE, cookie_for(&token))
        .send()
        .await
        .expect("send");
    assert_eq!(allowed.status(), StatusCode::OK);
    assert_eq!(server.hits.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn s7_rotating_the_token_invalidates_the_old_one_at_once() {
    let old: AccessToken = AccessToken::generate();
    let guard: AccessGuard = AccessGuard::new(old.clone(), true);
    let server: TestServer = spawn(guard.clone()).await;
    let url: String = format!("{}/hello", server.base);
    let first: Response = server
        .client
        .get(&url)
        .header(COOKIE, cookie_for(&old))
        .send()
        .await
        .expect("send");
    assert_eq!(first.status(), StatusCode::OK);

    let new: AccessToken = AccessToken::generate();
    guard.set_token(new.clone());
    let stale: Response = server
        .client
        .get(&url)
        .header(COOKIE, cookie_for(&old))
        .send()
        .await
        .expect("send");
    assert_eq!(stale.status(), StatusCode::UNAUTHORIZED);
    let fresh: Response = server
        .client
        .get(&url)
        .header(COOKIE, cookie_for(&new))
        .send()
        .await
        .expect("send");
    assert_eq!(fresh.status(), StatusCode::OK);
    assert_eq!(server.hits.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn s8_wrong_tokens_of_any_shape_get_the_same_answer() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token.clone(), true)).await;
    let url: String = format!("{}/hello", server.base);
    let almost: String = format!("{}0", &token.as_str()[..63]);
    let mut statuses: Vec<(StatusCode, String)> = Vec::new();
    for candidate in [almost.as_str(), "", "short"] {
        let response: Response = server
            .client
            .get(&url)
            .header(AUTHORIZATION, format!("Bearer {candidate}"))
            .send()
            .await
            .expect("send");
        let status: StatusCode = response.status();
        statuses.push((status, response.text().await.expect("body")));
    }
    assert!(
        statuses
            .iter()
            .all(|(status, body)| *status == StatusCode::UNAUTHORIZED && body.is_empty())
    );
    assert_eq!(server.hits.load(Ordering::SeqCst), 0);
}

#[tokio::test(flavor = "multi_thread")]
async fn login_path_with_another_method_still_needs_a_token() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token, true)).await;

    for method in [
        reqwest::Method::GET,
        reqwest::Method::PUT,
        reqwest::Method::DELETE,
    ] {
        let response: Response = server
            .client
            .request(method.clone(), format!("{}/access-login", server.base))
            .header(ACCEPT, "text/html")
            .send()
            .await
            .expect("request");
        assert_eq!(response.status(), StatusCode::UNAUTHORIZED, "{method}");
    }
    assert_eq!(server.hits.load(Ordering::SeqCst), 0);
}

#[tokio::test(flavor = "multi_thread")]
async fn token_redirect_never_points_to_another_host() {
    let token: AccessToken = AccessToken::generate();
    let server: TestServer = spawn(AccessGuard::new(token.clone(), true)).await;

    let response: Response = server
        .client
        .get(format!(
            "{}//evil.example/?token={}",
            server.base,
            token.as_str()
        ))
        .send()
        .await
        .expect("request");

    assert_eq!(response.status(), StatusCode::SEE_OTHER);
    assert_eq!(header(&response, LOCATION), "/evil.example/");
}

#[tokio::test(flavor = "multi_thread")]
async fn cross_site_writes_are_refused_before_reaching_the_app() {
    use axum::middleware::from_fn;
    use iworkflow::modules::preferences::infrastructure::adapters::inbound::same_origin::reject_cross_site_writes;

    let hits: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
    let app: Router = Router::new()
        .route("/hello", get(hello).post(hello))
        .with_state(hits.clone())
        .layer(from_fn(reject_cross_site_writes));
    let listener: tokio::net::TcpListener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind");
    let addr: SocketAddr = listener.local_addr().expect("addr");
    tokio::spawn(async move {
        axum::serve(listener, app).await.expect("serve");
    });
    let client: Client = Client::new();
    let url: String = format!("http://{addr}/hello");

    let foreign: Response = client
        .post(&url)
        .header("Origin", "https://evil.example")
        .send()
        .await
        .expect("request");
    assert_eq!(foreign.status(), StatusCode::FORBIDDEN);
    assert_eq!(hits.load(Ordering::SeqCst), 0);

    let same: Response = client
        .post(&url)
        .header("Origin", format!("http://{addr}"))
        .send()
        .await
        .expect("request");
    assert_eq!(same.status(), StatusCode::OK);
    let read: Response = client
        .get(&url)
        .header("Origin", "https://evil.example")
        .send()
        .await
        .expect("request");
    assert_eq!(read.status(), StatusCode::OK);
}
