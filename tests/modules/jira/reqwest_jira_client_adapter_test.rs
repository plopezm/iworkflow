use std::sync::{Arc, Mutex};

use axum::extract::{Path, Query, State};
use axum::http::{HeaderMap, StatusCode};
use axum::routing::get;
use axum::{Json, Router};
use iworkflow::modules::jira::domain::errors::jira_error::JiraError;
use iworkflow::modules::jira::domain::models::jira_comment::JiraComment;
use iworkflow::modules::jira::domain::models::jira_connection::JiraConnection;
use iworkflow::modules::jira::domain::models::jira_issue::RemoteIssue;
use iworkflow::modules::jira::domain::models::jira_transition::JiraTransition;
use iworkflow::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use iworkflow::modules::jira::infrastructure::adapters::outbound::api::reqwest_jira_client_adapter::ReqwestJiraClientAdapter;
use serde_json::{Value, json};

const TOKEN: &str = "pat-123";

#[derive(Default)]
struct Recorded {
    posts: Vec<(String, Value)>,
    search_calls: Vec<(String, String)>,
}

type Shared = Arc<Mutex<Recorded>>;

fn authorized(headers: &HeaderMap) -> bool {
    headers
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .is_some_and(|v| v == format!("Bearer {TOKEN}"))
}

fn issue_json(key: &str) -> Value {
    json!({"key": key, "fields": {
        "summary": format!("resumen {key}"), "description": "desc",
        "status": {"name": "To Do"}, "issuetype": {"name": "Bug"},
        "priority": {"name": "High"}, "labels": [],
        "updated": "2024-05-01T10:00:00.000+0000"}})
}

async fn myself(headers: HeaderMap) -> Result<Json<Value>, StatusCode> {
    if !authorized(&headers) {
        return Err(StatusCode::UNAUTHORIZED);
    }
    Ok(Json(json!({"displayName": "Ana Pérez"})))
}

async fn search(
    State(shared): State<Shared>,
    Query(params): Query<std::collections::HashMap<String, String>>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    let jql: String = params.get("jql").cloned().unwrap_or_default();
    let start: usize = params
        .get("startAt")
        .and_then(|s| s.parse().ok())
        .unwrap_or(0);
    shared
        .lock()
        .expect("lock")
        .search_calls
        .push((jql.clone(), start.to_string()));
    if jql == "broken" {
        return Err((
            StatusCode::BAD_REQUEST,
            Json(json!({"errorMessages": ["JQL inválido"]})),
        ));
    }
    // Three issues, served two per page.
    let all: [&str; 3] = ["A-1", "A-2", "A-3"];
    let page: Vec<Value> = all
        .iter()
        .skip(start)
        .take(2)
        .map(|k| issue_json(k))
        .collect();
    Ok(Json(json!({"startAt": start, "total": 3, "issues": page})))
}

async fn issue(Path(key): Path<String>) -> Result<Json<Value>, StatusCode> {
    if key == "NOPE-1" {
        return Err(StatusCode::NOT_FOUND);
    }
    Ok(Json(issue_json(&key)))
}

async fn comments(Path(_key): Path<String>) -> Json<Value> {
    Json(
        json!({"comments": [{"id": "9", "author": {"displayName": "Luis"}, "body": "hola",
        "created": "2024-05-01T10:00:00.000+0000"}]}),
    )
}

async fn add_comment(
    State(shared): State<Shared>,
    Path(key): Path<String>,
    Json(body): Json<Value>,
) -> StatusCode {
    shared
        .lock()
        .expect("lock")
        .posts
        .push((format!("comment:{key}"), body));
    StatusCode::CREATED
}

async fn transitions(Path(_key): Path<String>) -> Json<Value> {
    Json(json!({"transitions": [{"id": "31", "name": "Cerrar", "to": {"name": "Done"}}]}))
}

async fn do_transition(
    State(shared): State<Shared>,
    Path(key): Path<String>,
    Json(body): Json<Value>,
) -> StatusCode {
    shared
        .lock()
        .expect("lock")
        .posts
        .push((format!("transition:{key}"), body));
    StatusCode::NO_CONTENT
}

async fn start_server() -> (String, Shared) {
    let shared: Shared = Arc::default();
    let router: Router = Router::new()
        .route("/rest/api/2/myself", get(myself))
        .route("/rest/api/2/search", get(search))
        .route("/rest/api/2/issue/{key}", get(issue))
        .route(
            "/rest/api/2/issue/{key}/comment",
            get(comments).post(add_comment),
        )
        .route(
            "/rest/api/2/issue/{key}/transitions",
            get(transitions).post(do_transition),
        )
        .with_state(shared.clone());
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind");
    let url: String = format!("http://{}", listener.local_addr().expect("addr"));
    tokio::spawn(async move {
        let _ = axum::serve(listener, router).await;
    });
    (url, shared)
}

fn connection(url: &str, token: &str) -> JiraConnection {
    JiraConnection::new(url, token, false).expect("valid")
}

#[tokio::test(flavor = "multi_thread")]
async fn verify_returns_the_user_name_with_a_bearer_token() {
    let (url, _) = start_server().await;
    let client: ReqwestJiraClientAdapter = ReqwestJiraClientAdapter::new().expect("client");
    assert_eq!(
        client.verify(&connection(&url, TOKEN)).await.as_deref(),
        Ok("Ana Pérez")
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn a_rejected_token_is_unauthorized() {
    let (url, _) = start_server().await;
    let client: ReqwestJiraClientAdapter = ReqwestJiraClientAdapter::new().expect("client");
    assert_eq!(
        client.verify(&connection(&url, "wrong")).await,
        Err(JiraError::Unauthorized)
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn an_unreachable_server_is_a_remote_error() {
    let client: ReqwestJiraClientAdapter = ReqwestJiraClientAdapter::new().expect("client");
    let result: Result<String, JiraError> = client
        .verify(&connection("http://127.0.0.1:1", TOKEN))
        .await;
    assert!(matches!(result, Err(JiraError::Remote(_))), "{result:?}");
}

#[tokio::test(flavor = "multi_thread")]
async fn search_follows_pagination_and_sends_the_jql() {
    let (url, shared) = start_server().await;
    let client: ReqwestJiraClientAdapter = ReqwestJiraClientAdapter::new().expect("client");
    let issues: Vec<RemoteIssue> = client
        .search(&connection(&url, TOKEN), "project = X")
        .await
        .expect("search");
    let keys: Vec<&str> = issues
        .iter()
        .map(|i: &RemoteIssue| i.key.as_str())
        .collect();
    assert_eq!(keys, vec!["A-1", "A-2", "A-3"]);
    assert_eq!(issues[0].url, format!("{url}/browse/A-1"));
    let calls = shared.lock().expect("lock").search_calls.clone();
    assert_eq!(
        calls,
        vec![
            ("project = X".to_string(), "0".to_string()),
            ("project = X".to_string(), "2".to_string())
        ]
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn a_jira_error_body_becomes_the_error_message() {
    let (url, _) = start_server().await;
    let client: ReqwestJiraClientAdapter = ReqwestJiraClientAdapter::new().expect("client");
    assert_eq!(
        client.search(&connection(&url, TOKEN), "broken").await,
        Err(JiraError::Remote("HTTP 400: JQL inválido".to_string()))
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn get_issue_maps_found_and_missing() {
    let (url, _) = start_server().await;
    let client: ReqwestJiraClientAdapter = ReqwestJiraClientAdapter::new().expect("client");
    let found: RemoteIssue = client
        .get_issue(&connection(&url, TOKEN), "B-2")
        .await
        .expect("get");
    assert_eq!(found.summary, "resumen B-2");
    assert_eq!(
        client.get_issue(&connection(&url, TOKEN), "NOPE-1").await,
        Err(JiraError::IssueNotFound("NOPE-1".to_string()))
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn comments_and_transitions_are_listed() {
    let (url, _) = start_server().await;
    let client: ReqwestJiraClientAdapter = ReqwestJiraClientAdapter::new().expect("client");
    let comments: Vec<JiraComment> = client
        .comments(&connection(&url, TOKEN), "A-1")
        .await
        .expect("c");
    assert_eq!(comments[0].author, "Luis");
    let transitions: Vec<JiraTransition> = client
        .transitions(&connection(&url, TOKEN), "A-1")
        .await
        .expect("t");
    assert_eq!(transitions[0].id, "31");
    assert_eq!(transitions[0].to_status, "Done");
}

#[tokio::test(flavor = "multi_thread")]
async fn comment_and_transition_post_the_expected_bodies() {
    let (url, shared) = start_server().await;
    let client: ReqwestJiraClientAdapter = ReqwestJiraClientAdapter::new().expect("client");
    client
        .add_comment(&connection(&url, TOKEN), "A-1", "listo")
        .await
        .expect("comment");
    client
        .transition(&connection(&url, TOKEN), "A-1", "31")
        .await
        .expect("transition");
    let posts = shared.lock().expect("lock").posts.clone();
    assert_eq!(
        posts,
        vec![
            ("comment:A-1".to_string(), json!({"body": "listo"})),
            (
                "transition:A-1".to_string(),
                json!({"transition": {"id": "31"}})
            ),
        ]
    );
}
