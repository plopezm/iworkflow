use std::sync::Arc;

use reqwest::{Method, RequestBuilder, Response, StatusCode};
use serde::Deserialize;
use serde_json::{Value, json};

use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use crate::modules::gitlab::domain::models::merge_request::{
    MergeRequest, NewMergeRequest, Pipeline, PipelineJob,
};
use crate::modules::gitlab::domain::ports::outbound::gitlab_client_port::GitLabClientPort;
use crate::modules::gitlab::infrastructure::adapters::outbound::http::http_client_manager::HttpClientManager;

const TOKEN_HEADER: &str = "PRIVATE-TOKEN";
const JOBS_PER_PAGE: &str = "100";

/// Cliente de la API REST v4 de GitLab sobre `reqwest`.
#[derive(Debug, Default)]
pub struct GitLabHttpClientAdapter {
    clients: Arc<HttpClientManager>,
}

impl GitLabHttpClientAdapter {
    pub fn new() -> Self {
        Self::default()
    }

    /// Usa el gestor compartido de clientes (un único pool para toda la aplicación).
    pub fn with_clients(clients: Arc<HttpClientManager>) -> Self {
        Self { clients }
    }

    fn request(
        &self,
        settings: &GitLabSettings,
        method: Method,
        path: &str,
    ) -> Result<RequestBuilder, GitLabError> {
        let url: String = format!("{}{}", settings.api_url(), path);
        Ok(self
            .clients
            .client(settings.insecure_tls)?
            .request(method, url)
            .header(TOKEN_HEADER, settings.token()))
    }

    async fn send(request: RequestBuilder) -> Result<Response, GitLabError> {
        let response: Response = request
            .send()
            .await
            .map_err(|e: reqwest::Error| GitLabError::Network(e.without_url().to_string()))?;
        let status: StatusCode = response.status();
        if status.is_success() {
            return Ok(response);
        }
        if status.is_redirection() {
            return Err(GitLabError::Redirected(status.as_u16()));
        }
        Err(api_error(response).await)
    }

    async fn json<T: for<'de> Deserialize<'de>>(response: Response) -> Result<T, GitLabError> {
        response
            .json::<T>()
            .await
            .map_err(|e: reqwest::Error| GitLabError::Network(e.without_url().to_string()))
    }
}

/// Codifica un componente de ruta (el proyecto `g/p` o una rama `pi/x`) para la API.
fn encode_component(value: &str) -> String {
    let mut encoded: String = String::with_capacity(value.len());
    for byte in value.bytes() {
        if byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.' | b'~') {
            encoded.push(char::from(byte));
        } else {
            encoded.push_str(&format!("%{byte:02X}"));
        }
    }
    encoded
}

async fn api_error(response: Response) -> GitLabError {
    let status: StatusCode = response.status();
    let body: String = response.text().await.unwrap_or_default();
    GitLabError::Api {
        status: status.as_u16(),
        message: error_message(&body, status),
    }
}

/// GitLab devuelve `{"message": ...}` (texto, lista u objeto) o `{"error": ...}`.
fn error_message(body: &str, status: StatusCode) -> String {
    let parsed: Option<Value> = serde_json::from_str(body).ok();
    let message: Option<String> = parsed.and_then(|v: Value| {
        let field: &Value = v.get("message").or_else(|| v.get("error"))?;
        Some(match field {
            Value::String(text) => text.clone(),
            other => other.to_string(),
        })
    });
    message.unwrap_or_else(|| {
        if body.trim().is_empty() {
            status.canonical_reason().unwrap_or("error").to_string()
        } else {
            body.chars().take(300).collect()
        }
    })
}

#[derive(Deserialize)]
struct MergeRequestJson {
    iid: u64,
    title: String,
    state: String,
    source_branch: String,
    target_branch: String,
    web_url: String,
    merge_status: Option<String>,
    detailed_merge_status: Option<String>,
    has_conflicts: Option<bool>,
}

impl From<MergeRequestJson> for MergeRequest {
    fn from(json: MergeRequestJson) -> Self {
        Self {
            iid: json.iid,
            title: json.title,
            state: json.state,
            source_branch: json.source_branch,
            target_branch: json.target_branch,
            web_url: json.web_url,
            merge_status: json
                .detailed_merge_status
                .or(json.merge_status)
                .unwrap_or_default(),
            has_conflicts: json.has_conflicts.unwrap_or(false),
        }
    }
}

#[derive(Deserialize)]
struct PipelineJson {
    id: u64,
    status: String,
    web_url: String,
}

#[derive(Deserialize)]
struct JobJson {
    id: u64,
    name: String,
    stage: String,
    status: String,
    web_url: String,
}

#[derive(Deserialize)]
struct UserJson {
    username: String,
}

#[async_trait::async_trait]
impl GitLabClientPort for GitLabHttpClientAdapter {
    async fn current_user(&self, settings: &GitLabSettings) -> Result<String, GitLabError> {
        let response: Response = Self::send(self.request(settings, Method::GET, "/user")?).await?;
        Ok(Self::json::<UserJson>(response).await?.username)
    }

    async fn branch_exists(
        &self,
        settings: &GitLabSettings,
        project: &str,
        branch: &str,
    ) -> Result<bool, GitLabError> {
        let path: String = format!(
            "/projects/{}/repository/branches/{}",
            encode_component(project),
            encode_component(branch)
        );
        match Self::send(self.request(settings, Method::GET, &path)?).await {
            Ok(_) => Ok(true),
            Err(GitLabError::Api { status: 404, .. }) => Ok(false),
            Err(other) => Err(other),
        }
    }

    async fn find_latest_merge_request(
        &self,
        settings: &GitLabSettings,
        project: &str,
        source_branch: &str,
    ) -> Result<Option<MergeRequest>, GitLabError> {
        let path: String = format!("/projects/{}/merge_requests", encode_component(project));
        let request: RequestBuilder = self.request(settings, Method::GET, &path)?.query(&[
            ("source_branch", source_branch),
            ("order_by", "created_at"),
            ("sort", "desc"),
        ]);
        let response: Response = Self::send(request).await?;
        let list: Vec<MergeRequestJson> = Self::json(response).await?;
        // Prefiere el abierto más reciente; si no hay, el más reciente de cualquier estado.
        let index: usize = list
            .iter()
            .position(|mr: &MergeRequestJson| mr.state == "opened")
            .unwrap_or(0);
        Ok(list.into_iter().nth(index).map(MergeRequest::from))
    }

    async fn create_merge_request(
        &self,
        settings: &GitLabSettings,
        project: &str,
        new: &NewMergeRequest,
    ) -> Result<MergeRequest, GitLabError> {
        let path: String = format!("/projects/{}/merge_requests", encode_component(project));
        let request: RequestBuilder = self.request(settings, Method::POST, &path)?.json(&json!({
            "source_branch": new.source_branch,
            "target_branch": new.target_branch,
            "title": new.title,
            "description": new.description,
        }));
        let response: Response = Self::send(request).await?;
        Ok(Self::json::<MergeRequestJson>(response).await?.into())
    }

    async fn get_merge_request(
        &self,
        settings: &GitLabSettings,
        project: &str,
        iid: u64,
    ) -> Result<MergeRequest, GitLabError> {
        let path: String = format!(
            "/projects/{}/merge_requests/{iid}",
            encode_component(project)
        );
        let response: Response = Self::send(self.request(settings, Method::GET, &path)?).await?;
        Ok(Self::json::<MergeRequestJson>(response).await?.into())
    }

    async fn latest_pipeline(
        &self,
        settings: &GitLabSettings,
        project: &str,
        iid: u64,
    ) -> Result<Option<Pipeline>, GitLabError> {
        let encoded: String = encode_component(project);
        let list_path: String = format!("/projects/{encoded}/merge_requests/{iid}/pipelines");
        let response: Response =
            Self::send(self.request(settings, Method::GET, &list_path)?).await?;
        let pipelines: Vec<PipelineJson> = Self::json(response).await?;
        // GitLab los devuelve del más reciente al más antiguo.
        let Some(latest) = pipelines.into_iter().next() else {
            return Ok(None);
        };
        let jobs_path: String = format!("/projects/{encoded}/pipelines/{}/jobs", latest.id);
        let jobs_request: RequestBuilder = self
            .request(settings, Method::GET, &jobs_path)?
            .query(&[("per_page", JOBS_PER_PAGE)]);
        let jobs_response: Response = Self::send(jobs_request).await?;
        let jobs: Vec<JobJson> = Self::json(jobs_response).await?;
        Ok(Some(Pipeline {
            id: latest.id,
            status: latest.status,
            web_url: latest.web_url,
            jobs: jobs
                .into_iter()
                .map(|job: JobJson| PipelineJob {
                    id: job.id,
                    name: job.name,
                    stage: job.stage,
                    status: job.status,
                    web_url: job.web_url,
                })
                .collect(),
        }))
    }

    async fn merge_merge_request(
        &self,
        settings: &GitLabSettings,
        project: &str,
        iid: u64,
    ) -> Result<MergeRequest, GitLabError> {
        let path: String = format!(
            "/projects/{}/merge_requests/{iid}/merge",
            encode_component(project)
        );
        let response: Response = Self::send(self.request(settings, Method::PUT, &path)?).await?;
        Ok(Self::json::<MergeRequestJson>(response).await?.into())
    }
}

#[cfg(test)]
mod tests {
    use std::sync::{Arc, Mutex};

    use axum::Router;
    use axum::extract::State;
    use axum::http::{HeaderMap, Method as HttpMethod, StatusCode as HttpStatus, Uri};
    use axum::response::{IntoResponse, Response as AxumResponse};
    use tokio::net::TcpListener;

    use super::*;

    const TOKEN: &str = "glpat-test";

    #[derive(Debug, Clone, PartialEq, Eq)]
    struct Recorded {
        method: String,
        target: String,
        token: Option<String>,
        body: String,
    }

    #[derive(Clone)]
    struct Scripted {
        routes: Arc<Vec<(String, u16, String)>>,
        seen: Arc<Mutex<Vec<Recorded>>>,
    }

    async fn handle(
        State(state): State<Scripted>,
        method: HttpMethod,
        uri: Uri,
        headers: HeaderMap,
        body: String,
    ) -> AxumResponse {
        let target: String = uri
            .path_and_query()
            .map(|p| p.as_str().to_string())
            .unwrap_or_default();
        let key: String = format!("{method} {target}");
        if let Ok(mut seen) = state.seen.lock() {
            seen.push(Recorded {
                method: method.to_string(),
                target,
                token: headers
                    .get("private-token")
                    .and_then(|v| v.to_str().ok())
                    .map(str::to_string),
                body,
            });
        }
        match state.routes.iter().find(|(k, _, _)| *k == key) {
            Some((_, status, payload)) => (
                HttpStatus::from_u16(*status).unwrap_or(HttpStatus::OK),
                [("content-type", "application/json")],
                payload.clone(),
            )
                .into_response(),
            None => (HttpStatus::NOT_FOUND, r#"{"message":"404 Not Found"}"#).into_response(),
        }
    }

    /// Levanta un GitLab falso; `routes` = (`"METHOD /ruta?query"`, estado, cuerpo JSON).
    async fn fake_gitlab(
        routes: Vec<(&str, u16, &str)>,
    ) -> (GitLabSettings, Arc<Mutex<Vec<Recorded>>>) {
        let seen: Arc<Mutex<Vec<Recorded>>> = Arc::new(Mutex::new(Vec::new()));
        let state: Scripted = Scripted {
            routes: Arc::new(
                routes
                    .into_iter()
                    .map(|(k, s, b)| (k.to_string(), s, b.to_string()))
                    .collect(),
            ),
            seen: seen.clone(),
        };
        let app: Router = Router::new().fallback(handle).with_state(state);
        let listener: TcpListener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let addr: std::net::SocketAddr = listener.local_addr().expect("addr");
        tokio::spawn(async move {
            let _ = axum::serve(listener, app).await;
        });
        let settings: GitLabSettings =
            GitLabSettings::new(&format!("http://{addr}"), TOKEN, false).expect("settings");
        (settings, seen)
    }

    const MR: &str = r#"{"iid":7,"title":"T","state":"opened","source_branch":"pi/x","target_branch":"main","web_url":"http://gl/mr/7","merge_status":"can_be_merged","detailed_merge_status":"mergeable","has_conflicts":false}"#;

    #[test]
    fn encodes_slashes_and_reserved_characters() {
        assert_eq!(
            encode_component("grupo/sub/p.q-r_s"),
            "grupo%2Fsub%2Fp.q-r_s"
        );
        assert_eq!(encode_component("a b#c"), "a%20b%23c");
    }

    #[tokio::test]
    async fn current_user_sends_token_and_returns_username() {
        let (settings, seen) =
            fake_gitlab(vec![("GET /api/v4/user", 200, r#"{"username":"pablo"}"#)]).await;

        let user: String = GitLabHttpClientAdapter::new()
            .current_user(&settings)
            .await
            .expect("user");

        assert_eq!(user, "pablo");
        let seen = seen.lock().expect("lock");
        assert_eq!(seen[0].token.as_deref(), Some(TOKEN));
    }

    #[tokio::test]
    async fn branch_exists_maps_404_to_false() {
        let (settings, _) = fake_gitlab(vec![(
            "GET /api/v4/projects/g%2Fp/repository/branches/pi%2Fx",
            200,
            r#"{"name":"pi/x"}"#,
        )])
        .await;
        let client: GitLabHttpClientAdapter = GitLabHttpClientAdapter::new();

        assert!(
            client
                .branch_exists(&settings, "g/p", "pi/x")
                .await
                .expect("ok")
        );
        assert!(
            !client
                .branch_exists(&settings, "g/p", "otra")
                .await
                .expect("ok")
        );
    }

    #[tokio::test]
    async fn find_latest_merge_request_returns_first_or_none() {
        let list: String = format!("[{MR}]");
        let (settings, _) = fake_gitlab(vec![
            (
                "GET /api/v4/projects/g%2Fp/merge_requests?source_branch=pi%2Fx&order_by=created_at&sort=desc",
                200,
                list.as_str(),
            ),
            (
                "GET /api/v4/projects/g%2Fp/merge_requests?source_branch=vacia&order_by=created_at&sort=desc",
                200,
                "[]",
            ),
        ])
        .await;
        let client: GitLabHttpClientAdapter = GitLabHttpClientAdapter::new();

        let found: Option<MergeRequest> = client
            .find_latest_merge_request(&settings, "g/p", "pi/x")
            .await
            .expect("find");
        let missing: Option<MergeRequest> = client
            .find_latest_merge_request(&settings, "g/p", "vacia")
            .await
            .expect("find");

        let mr: MergeRequest = found.expect("mr");
        assert_eq!((mr.iid, mr.merge_status.as_str()), (7, "mergeable"));
        assert!(mr.is_open());
        assert_eq!(missing, None);
    }

    #[tokio::test]
    async fn create_merge_request_posts_expected_body() {
        let (settings, seen) = fake_gitlab(vec![(
            "POST /api/v4/projects/g%2Fp/merge_requests",
            201,
            MR,
        )])
        .await;

        let created: MergeRequest = GitLabHttpClientAdapter::new()
            .create_merge_request(
                &settings,
                "g/p",
                &NewMergeRequest {
                    source_branch: "pi/x".to_string(),
                    target_branch: "main".to_string(),
                    title: "T".to_string(),
                    description: "D".to_string(),
                },
            )
            .await
            .expect("create");

        assert_eq!(created.web_url, "http://gl/mr/7");
        let body: Value = serde_json::from_str(&seen.lock().expect("lock")[0].body).expect("json");
        assert_eq!(
            body,
            json!({"source_branch":"pi/x","target_branch":"main","title":"T","description":"D"})
        );
    }

    #[tokio::test]
    async fn latest_pipeline_returns_newest_with_jobs() {
        let (settings, _) = fake_gitlab(vec![
            (
                "GET /api/v4/projects/g%2Fp/merge_requests/7/pipelines",
                200,
                r#"[{"id":22,"status":"running","web_url":"http://gl/p/22"},{"id":21,"status":"failed","web_url":"http://gl/p/21"}]"#,
            ),
            (
                "GET /api/v4/projects/g%2Fp/pipelines/22/jobs?per_page=100",
                200,
                r#"[{"id":1,"name":"build","stage":"build","status":"success","web_url":"http://gl/j/1"}]"#,
            ),
        ])
        .await;

        let pipeline: Pipeline = GitLabHttpClientAdapter::new()
            .latest_pipeline(&settings, "g/p", 7)
            .await
            .expect("pipeline")
            .expect("some");

        assert_eq!((pipeline.id, pipeline.status.as_str()), (22, "running"));
        assert_eq!(pipeline.jobs.len(), 1);
        assert_eq!(pipeline.jobs[0].name, "build");
    }

    #[tokio::test]
    async fn latest_pipeline_is_none_without_pipelines() {
        let (settings, _) = fake_gitlab(vec![(
            "GET /api/v4/projects/g%2Fp/merge_requests/7/pipelines",
            200,
            "[]",
        )])
        .await;

        let pipeline: Option<Pipeline> = GitLabHttpClientAdapter::new()
            .latest_pipeline(&settings, "g/p", 7)
            .await
            .expect("pipeline");

        assert_eq!(pipeline, None);
    }

    #[tokio::test]
    async fn merge_puts_and_surfaces_gitlab_error_message() {
        let (settings, seen) = fake_gitlab(vec![
            ("PUT /api/v4/projects/g%2Fp/merge_requests/7/merge", 200, MR),
            (
                "PUT /api/v4/projects/g%2Fp/merge_requests/8/merge",
                405,
                r#"{"message":"405 Method Not Allowed"}"#,
            ),
        ])
        .await;
        let client: GitLabHttpClientAdapter = GitLabHttpClientAdapter::new();

        client
            .merge_merge_request(&settings, "g/p", 7)
            .await
            .expect("merge");
        let err: GitLabError = client
            .merge_merge_request(&settings, "g/p", 8)
            .await
            .unwrap_err();

        assert_eq!(
            err,
            GitLabError::Api {
                status: 405,
                message: "405 Method Not Allowed".to_string()
            }
        );
        assert_eq!(seen.lock().expect("lock").len(), 2);
    }

    #[tokio::test]
    async fn unauthorized_reports_error_field() {
        let (settings, _) = fake_gitlab(vec![(
            "GET /api/v4/user",
            401,
            r#"{"message":"401 Unauthorized"}"#,
        )])
        .await;

        let err: GitLabError = GitLabHttpClientAdapter::new()
            .current_user(&settings)
            .await
            .unwrap_err();

        assert!(matches!(err, GitLabError::Api { status: 401, .. }));
    }

    #[tokio::test]
    async fn network_failure_does_not_leak_token() {
        let settings: GitLabSettings =
            GitLabSettings::new("http://127.0.0.1:1", TOKEN, false).expect("settings");

        let err: GitLabError = GitLabHttpClientAdapter::new()
            .current_user(&settings)
            .await
            .unwrap_err();

        assert!(matches!(err, GitLabError::Network(_)));
        assert!(!err.to_string().contains(TOKEN));
    }
}

#[cfg(test)]
mod redirect_tests {
    use std::sync::{Arc, Mutex};

    use axum::Router;
    use axum::http::{HeaderMap, StatusCode as HttpStatus};
    use axum::routing::get;
    use tokio::net::TcpListener;

    use super::*;

    #[tokio::test]
    async fn redirect_to_another_host_never_receives_the_token() {
        let leaked: Arc<Mutex<Vec<String>>> = Arc::new(Mutex::new(Vec::new()));
        let sink: Arc<Mutex<Vec<String>>> = leaked.clone();
        let target: Router = Router::new().fallback(get(move |headers: HeaderMap| {
            let sink: Arc<Mutex<Vec<String>>> = sink.clone();
            async move {
                if let (Some(value), Ok(mut seen)) = (headers.get("private-token"), sink.lock()) {
                    seen.push(value.to_str().unwrap_or_default().to_string());
                }
                (HttpStatus::OK, r#"{"username":"intruso"}"#)
            }
        }));
        let target_listener: TcpListener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let target_addr: std::net::SocketAddr = target_listener.local_addr().expect("addr");
        tokio::spawn(async move {
            let _ = axum::serve(target_listener, target).await;
        });
        let location: String = format!("http://{target_addr}/api/v4/user");
        let origin: Router = Router::new().fallback(get(move || {
            let location: String = location.clone();
            async move { (HttpStatus::FOUND, [("location", location)], "") }
        }));
        let origin_listener: TcpListener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let origin_addr: std::net::SocketAddr = origin_listener.local_addr().expect("addr");
        tokio::spawn(async move {
            let _ = axum::serve(origin_listener, origin).await;
        });
        let settings: GitLabSettings =
            GitLabSettings::new(&format!("http://{origin_addr}"), "glpat-secret", false)
                .expect("settings");

        let result: Result<String, GitLabError> =
            GitLabHttpClientAdapter::new().current_user(&settings).await;

        assert_eq!(result, Err(GitLabError::Redirected(302)));
        assert!(leaked.lock().expect("lock").is_empty());
    }
}
