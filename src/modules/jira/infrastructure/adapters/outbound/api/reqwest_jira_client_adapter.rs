use std::time::Duration;

use reqwest::{Client, Method, RequestBuilder, Response, StatusCode};
use serde_json::{Value, json};

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_comment::JiraComment;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::models::jira_issue::{RemoteIssue, is_valid_issue_key};
use crate::modules::jira::domain::models::jira_transition::JiraTransition;
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::infrastructure::adapters::outbound::api::jira_json::{
    ISSUE_FIELDS, comments_from_json, error_message, issue_from_json, search_page_from_json,
    transitions_from_json,
};

const REQUEST_TIMEOUT: Duration = Duration::from_secs(30);
const PAGE_SIZE: u64 = 100;
/// Upper bound of imported issues per sync, so a broad JQL cannot flood the local database.
pub const MAX_IMPORTED_ISSUES: usize = 500;
const MAX_COMMENTS: u64 = 200;

/// Jira Server/Data Center REST v2 over HTTP(S) with a Bearer personal access token.
pub struct ReqwestJiraClientAdapter {
    strict: Client,
    lenient: Client,
}

impl ReqwestJiraClientAdapter {
    pub fn new() -> Result<Self, JiraError> {
        Ok(Self {
            strict: build_client(false)?,
            lenient: build_client(true)?,
        })
    }

    fn request(&self, connection: &JiraConnection, method: Method, path: &str) -> RequestBuilder {
        let client: &Client = if connection.accept_invalid_certs {
            &self.lenient
        } else {
            &self.strict
        };
        client
            .request(method, connection.api_url(path))
            .bearer_auth(&connection.token)
            .header(reqwest::header::ACCEPT, "application/json")
    }

    /// Sends the request and returns the status-checked response.
    async fn send(
        &self,
        builder: RequestBuilder,
        not_found: Option<&str>,
    ) -> Result<Response, JiraError> {
        let response: Response = builder
            .send()
            .await
            .map_err(|e: reqwest::Error| JiraError::Remote(describe(&e)))?;
        let status: StatusCode = response.status();
        if status.is_success() {
            return Ok(response);
        }
        if status == StatusCode::UNAUTHORIZED || status == StatusCode::FORBIDDEN {
            return Err(JiraError::Unauthorized);
        }
        if status == StatusCode::NOT_FOUND
            && let Some(key) = not_found
        {
            return Err(JiraError::IssueNotFound(key.to_string()));
        }
        let body: String = response.text().await.unwrap_or_default();
        let detail: String = error_message(&body).unwrap_or_else(|| status.to_string());
        Err(JiraError::Remote(format!(
            "HTTP {}: {detail}",
            status.as_u16()
        )))
    }

    async fn json(
        &self,
        builder: RequestBuilder,
        not_found: Option<&str>,
    ) -> Result<Value, JiraError> {
        self.send(builder, not_found)
            .await?
            .json::<Value>()
            .await
            .map_err(|e: reqwest::Error| JiraError::Remote(format!("respuesta no válida: {e}")))
    }
}

/// REST path of an issue resource. Rejects keys that are not plain `PROJ-123` so a crafted key
/// cannot redirect the request to another endpoint.
fn issue_path(key: &str, suffix: &str) -> Result<String, JiraError> {
    if !is_valid_issue_key(key) {
        return Err(JiraError::InvalidIssueKey(key.to_string()));
    }
    Ok(format!("issue/{key}{suffix}"))
}

fn build_client(accept_invalid_certs: bool) -> Result<Client, JiraError> {
    Client::builder()
        .timeout(REQUEST_TIMEOUT)
        .danger_accept_invalid_certs(accept_invalid_certs)
        .build()
        .map_err(|e: reqwest::Error| JiraError::Remote(e.to_string()))
}

fn describe(error: &reqwest::Error) -> String {
    if error.is_timeout() {
        "tiempo de espera agotado".to_string()
    } else if error.is_connect() {
        format!("no se pudo conectar ({error})")
    } else {
        error.to_string()
    }
}

#[async_trait::async_trait]
impl JiraClientPort for ReqwestJiraClientAdapter {
    async fn verify(&self, connection: &JiraConnection) -> Result<String, JiraError> {
        let me: Value = self
            .json(self.request(connection, Method::GET, "myself"), None)
            .await?;
        Ok(me
            .get("displayName")
            .or_else(|| me.get("name"))
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string())
    }

    async fn search(
        &self,
        connection: &JiraConnection,
        jql: &str,
    ) -> Result<Vec<RemoteIssue>, JiraError> {
        let mut all: Vec<RemoteIssue> = Vec::new();
        let mut start_at: u64 = 0;
        loop {
            let builder: RequestBuilder = self.request(connection, Method::GET, "search").query(&[
                ("jql", jql),
                ("fields", ISSUE_FIELDS),
                ("startAt", &start_at.to_string()),
                ("maxResults", &PAGE_SIZE.to_string()),
            ]);
            let page: Value = self.json(builder, None).await?;
            let (issues, next, total) = search_page_from_json(connection, &page)?;
            let received: usize = issues.len();
            all.extend(issues);
            if received == 0 || next >= total || all.len() >= MAX_IMPORTED_ISSUES {
                break;
            }
            start_at = next;
        }
        all.truncate(MAX_IMPORTED_ISSUES);
        Ok(all)
    }

    async fn get_issue(
        &self,
        connection: &JiraConnection,
        key: &str,
    ) -> Result<RemoteIssue, JiraError> {
        let builder: RequestBuilder = self
            .request(connection, Method::GET, &issue_path(key, "")?)
            .query(&[("fields", ISSUE_FIELDS)]);
        let doc: Value = self.json(builder, Some(key)).await?;
        issue_from_json(connection, &doc)
    }

    async fn comments(
        &self,
        connection: &JiraConnection,
        key: &str,
    ) -> Result<Vec<JiraComment>, JiraError> {
        let builder: RequestBuilder = self
            .request(connection, Method::GET, &issue_path(key, "/comment")?)
            .query(&[("maxResults", MAX_COMMENTS.to_string())]);
        Ok(comments_from_json(&self.json(builder, Some(key)).await?))
    }

    async fn add_comment(
        &self,
        connection: &JiraConnection,
        key: &str,
        body: &str,
    ) -> Result<(), JiraError> {
        let builder: RequestBuilder = self
            .request(connection, Method::POST, &issue_path(key, "/comment")?)
            .json(&json!({ "body": body }));
        self.send(builder, Some(key)).await.map(|_| ())
    }

    async fn transitions(
        &self,
        connection: &JiraConnection,
        key: &str,
    ) -> Result<Vec<JiraTransition>, JiraError> {
        let builder: RequestBuilder =
            self.request(connection, Method::GET, &issue_path(key, "/transitions")?);
        Ok(transitions_from_json(&self.json(builder, Some(key)).await?))
    }

    async fn transition(
        &self,
        connection: &JiraConnection,
        key: &str,
        transition_id: &str,
    ) -> Result<(), JiraError> {
        let builder: RequestBuilder = self
            .request(connection, Method::POST, &issue_path(key, "/transitions")?)
            .json(&json!({ "transition": { "id": transition_id } }));
        self.send(builder, Some(key)).await.map(|_| ())
    }
}
