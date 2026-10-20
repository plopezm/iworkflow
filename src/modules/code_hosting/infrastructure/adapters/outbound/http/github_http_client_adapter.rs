use std::sync::Arc;

use reqwest::{Method, RequestBuilder, Response};
use serde::Deserialize;
use serde_json::json;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::models::merge_request::{
    MergeRequest, NewMergeRequest, Pipeline, PipelineJob,
};
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_port::CodeHostClientPort;
use crate::modules::code_hosting::infrastructure::adapters::outbound::http::http_client_manager::HttpClientManager;
use crate::modules::code_hosting::infrastructure::adapters::outbound::http::http_support::{
    encode_component, json, send,
};

const PROVIDER: HostingProvider = HostingProvider::GitHub;
const ACCEPT: &str = "application/vnd.github+json";
const API_VERSION_HEADER: &str = "X-GitHub-Api-Version";
const API_VERSION: &str = "2022-11-28";
/// GitHub rechaza las peticiones sin `User-Agent`.
const USER_AGENT: &str = "iworkflow";
const PULLS_PER_PAGE: &str = "30";
const JOBS_PER_PAGE: &str = "100";

/// Cliente de la API REST v3 de GitHub (github.com o Enterprise) sobre `reqwest`.
#[derive(Debug, Default)]
pub struct GitHubHttpClientAdapter {
    clients: Arc<HttpClientManager>,
}

impl GitHubHttpClientAdapter {
    pub fn new() -> Self {
        Self::default()
    }

    /// Usa el gestor compartido de clientes (un único pool para toda la aplicación).
    pub fn with_clients(clients: Arc<HttpClientManager>) -> Self {
        Self { clients }
    }

    fn request(
        &self,
        settings: &CodeHostSettings,
        method: Method,
        path: &str,
    ) -> Result<RequestBuilder, CodeHostError> {
        let url: String = format!("{}{}", settings.api_url(), path);
        Ok(self
            .clients
            .client(settings.insecure_tls)?
            .request(method, url)
            .bearer_auth(settings.token())
            .header(reqwest::header::ACCEPT, ACCEPT)
            .header(API_VERSION_HEADER, API_VERSION)
            .header(reqwest::header::USER_AGENT, USER_AGENT))
    }

    async fn pull(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        number: u64,
    ) -> Result<PullJson, CodeHostError> {
        let path: String = format!("{}/pulls/{number}", repo_path(project));
        let response: Response =
            send(PROVIDER, self.request(settings, Method::GET, &path)?).await?;
        json(response).await
    }
}

/// `/repos/{owner}/{repo}`: cada segmento se codifica y la barra que los separa se conserva.
fn repo_path(project: &str) -> String {
    let segments: Vec<String> = project.split('/').map(encode_component).collect();
    format!("/repos/{}", segments.join("/"))
}

/// Una rama como `pi/x` se codifica por segmentos para conservar las barras.
fn ref_path(branch: &str) -> String {
    let segments: Vec<String> = branch.split('/').map(encode_component).collect();
    segments.join("/")
}

/// `open` → `opened`; cerrado y mezclado → `merged`; cerrado sin mezclar → `closed`.
fn map_pull_state(state: &str, merged: bool) -> String {
    match (state, merged) {
        ("open", _) => "opened",
        (_, true) => "merged",
        _ => "closed",
    }
    .to_string()
}

/// Traduce `mergeable_state` al vocabulario neutro (`mergeable`, `conflict`, ...).
fn map_merge_status(mergeable_state: Option<&str>) -> &'static str {
    match mergeable_state {
        Some("clean" | "behind" | "unstable" | "has_hooks") => "mergeable",
        Some("dirty") => "conflict",
        Some("blocked") => "blocked_status",
        Some("draft") => "not_approved",
        _ => "checking",
    }
}

/// Traduce el estado/conclusión de un run o job de Actions al vocabulario neutro.
fn map_ci_status(status: &str, conclusion: Option<&str>) -> &'static str {
    match status {
        "queued" | "waiting" | "pending" | "requested" => "pending",
        "in_progress" => "running",
        "completed" => match conclusion {
            Some("success") => "success",
            Some("failure" | "timed_out" | "startup_failure") => "failed",
            Some("cancelled") => "canceled",
            Some("skipped" | "neutral" | "stale") => "skipped",
            Some("action_required") => "manual",
            _ => "pending",
        },
        _ => "pending",
    }
}

#[derive(Deserialize)]
struct RefJson {
    #[serde(rename = "ref")]
    name: String,
    sha: String,
}

#[derive(Deserialize)]
struct PullJson {
    number: u64,
    title: String,
    state: String,
    html_url: String,
    merged: Option<bool>,
    merged_at: Option<String>,
    mergeable_state: Option<String>,
    head: RefJson,
    base: RefJson,
}

impl From<PullJson> for MergeRequest {
    fn from(pull: PullJson) -> Self {
        let merged: bool = pull.merged.unwrap_or(false) || pull.merged_at.is_some();
        Self {
            iid: pull.number,
            title: pull.title,
            state: map_pull_state(&pull.state, merged),
            source_branch: pull.head.name,
            target_branch: pull.base.name,
            web_url: pull.html_url,
            merge_status: map_merge_status(pull.mergeable_state.as_deref()).to_string(),
            has_conflicts: pull.mergeable_state.as_deref() == Some("dirty"),
        }
    }
}

#[derive(Deserialize)]
struct UserJson {
    login: String,
}

#[derive(Deserialize)]
struct RunsJson {
    workflow_runs: Vec<RunJson>,
}

#[derive(Deserialize)]
struct RunJson {
    id: u64,
    name: Option<String>,
    status: Option<String>,
    conclusion: Option<String>,
    html_url: String,
}

#[derive(Deserialize)]
struct JobsJson {
    jobs: Vec<JobJson>,
}

#[derive(Deserialize)]
struct JobJson {
    id: u64,
    name: String,
    status: String,
    conclusion: Option<String>,
    html_url: Option<String>,
}

#[async_trait::async_trait]
impl CodeHostClientPort for GitHubHttpClientAdapter {
    async fn current_user(&self, settings: &CodeHostSettings) -> Result<String, CodeHostError> {
        let response: Response =
            send(PROVIDER, self.request(settings, Method::GET, "/user")?).await?;
        Ok(json::<UserJson>(response).await?.login)
    }

    async fn branch_exists(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        branch: &str,
    ) -> Result<bool, CodeHostError> {
        let path: String = format!("{}/git/ref/heads/{}", repo_path(project), ref_path(branch));
        match send(PROVIDER, self.request(settings, Method::GET, &path)?).await {
            Ok(_) => Ok(true),
            Err(CodeHostError::Api { status: 404, .. }) => Ok(false),
            Err(other) => Err(other),
        }
    }

    async fn find_latest_merge_request(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        source_branch: &str,
    ) -> Result<Option<MergeRequest>, CodeHostError> {
        let owner: &str = project.split('/').next().unwrap_or_default();
        let head: String = format!("{owner}:{source_branch}");
        let path: String = format!("{}/pulls", repo_path(project));
        let request: RequestBuilder = self.request(settings, Method::GET, &path)?.query(&[
            ("head", head.as_str()),
            ("state", "all"),
            ("sort", "created"),
            ("direction", "desc"),
            ("per_page", PULLS_PER_PAGE),
        ]);
        let response: Response = send(PROVIDER, request).await?;
        let list: Vec<PullJson> = json(response).await?;
        // Prefiere el abierto más reciente; si no hay, el más reciente de cualquier estado.
        let index: usize = list
            .iter()
            .position(|pull: &PullJson| pull.state == "open")
            .unwrap_or(0);
        let Some(chosen) = list.into_iter().nth(index) else {
            return Ok(None);
        };
        // El listado no trae los campos de mergeabilidad: se relee el PR elegido.
        Ok(Some(
            self.pull(settings, project, chosen.number).await?.into(),
        ))
    }

    async fn create_merge_request(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        new: &NewMergeRequest,
    ) -> Result<MergeRequest, CodeHostError> {
        let path: String = format!("{}/pulls", repo_path(project));
        let request: RequestBuilder = self.request(settings, Method::POST, &path)?.json(&json!({
            "title": new.title,
            "head": new.source_branch,
            "base": new.target_branch,
            "body": new.description,
        }));
        let response: Response = send(PROVIDER, request).await.map_err(|e: CodeHostError| {
            no_commits_error(e, &new.source_branch, &new.target_branch)
        })?;
        Ok(json::<PullJson>(response).await?.into())
    }

    async fn get_merge_request(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        iid: u64,
    ) -> Result<MergeRequest, CodeHostError> {
        Ok(self.pull(settings, project, iid).await?.into())
    }

    async fn latest_pipeline(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        iid: u64,
    ) -> Result<Option<Pipeline>, CodeHostError> {
        let sha: String = self.pull(settings, project, iid).await?.head.sha;
        let repo: String = repo_path(project);
        let runs_path: String = format!("{repo}/actions/runs");
        let runs_request: RequestBuilder = self
            .request(settings, Method::GET, &runs_path)?
            .query(&[("head_sha", sha.as_str()), ("per_page", "1")]);
        let runs_response: Response = send(PROVIDER, runs_request).await?;
        let runs: RunsJson = json(runs_response).await?;
        // GitHub los devuelve del más reciente al más antiguo.
        let Some(run) = runs.workflow_runs.into_iter().next() else {
            return Ok(None);
        };
        let jobs_path: String = format!("{repo}/actions/runs/{}/jobs", run.id);
        let jobs_request: RequestBuilder = self
            .request(settings, Method::GET, &jobs_path)?
            .query(&[("per_page", JOBS_PER_PAGE)]);
        let jobs_response: Response = send(PROVIDER, jobs_request).await?;
        let jobs: JobsJson = json(jobs_response).await?;
        let stage: String = run.name.clone().unwrap_or_default();
        Ok(Some(Pipeline {
            id: run.id,
            status: map_ci_status(
                run.status.as_deref().unwrap_or_default(),
                run.conclusion.as_deref(),
            )
            .to_string(),
            web_url: run.html_url.clone(),
            jobs: jobs
                .jobs
                .into_iter()
                .map(|job: JobJson| PipelineJob {
                    id: job.id,
                    name: job.name,
                    stage: stage.clone(),
                    status: map_ci_status(&job.status, job.conclusion.as_deref()).to_string(),
                    web_url: job.html_url.unwrap_or_else(|| run.html_url.clone()),
                })
                .collect(),
        }))
    }

    async fn merge_merge_request(
        &self,
        settings: &CodeHostSettings,
        project: &str,
        iid: u64,
    ) -> Result<MergeRequest, CodeHostError> {
        let path: String = format!("{}/pulls/{iid}/merge", repo_path(project));
        let request: RequestBuilder = self.request(settings, Method::PUT, &path)?.json(&json!({}));
        send(PROVIDER, request).await?;
        // La respuesta del merge no es un PR: se relee para devolver su estado final.
        Ok(self.pull(settings, project, iid).await?.into())
    }
}

/// GitHub rechaza con 422 «No commits between» un PR cuya rama no avanza sobre la base.
fn no_commits_error(error: CodeHostError, branch: &str, base: &str) -> CodeHostError {
    match error {
        CodeHostError::Api {
            status: 422,
            ref message,
            ..
        } if message.contains("No commits between") => CodeHostError::NoCommitsToPropose {
            branch: branch.to_string(),
            base: base.to_string(),
        },
        other => other,
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex;

    use axum::Router;
    use axum::extract::State;
    use axum::http::{HeaderMap, Method as HttpMethod, StatusCode as HttpStatus, Uri};
    use axum::response::{IntoResponse, Response as AxumResponse};
    use serde_json::Value;
    use tokio::net::TcpListener;

    use super::*;

    const TOKEN: &str = "ghp_test";

    #[derive(Debug, Clone, PartialEq, Eq)]
    struct Recorded {
        method: String,
        target: String,
        authorization: Option<String>,
        user_agent: Option<String>,
        accept: Option<String>,
        api_version: Option<String>,
        body: String,
    }

    #[derive(Clone)]
    struct Scripted {
        routes: Arc<Vec<(String, u16, String)>>,
        seen: Arc<Mutex<Vec<Recorded>>>,
    }

    fn header(headers: &HeaderMap, name: &str) -> Option<String> {
        headers
            .get(name)
            .and_then(|v| v.to_str().ok())
            .map(str::to_string)
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
                authorization: header(&headers, "authorization"),
                user_agent: header(&headers, "user-agent"),
                accept: header(&headers, "accept"),
                api_version: header(&headers, "x-github-api-version"),
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
            None => (HttpStatus::NOT_FOUND, r#"{"message":"Not Found"}"#).into_response(),
        }
    }

    /// Levanta un GitHub falso; `routes` = (`"METHOD /ruta?query"`, estado, cuerpo JSON).
    async fn fake_github(
        routes: Vec<(&str, u16, &str)>,
    ) -> (CodeHostSettings, Arc<Mutex<Vec<Recorded>>>) {
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
        // Un host distinto de github.com se trata como Enterprise: la API cuelga de `/api/v3`.
        let settings: CodeHostSettings = CodeHostSettings::new(
            HostingProvider::GitHub,
            &format!("http://{addr}"),
            TOKEN,
            false,
        )
        .expect("settings");
        (settings, seen)
    }

    const PULL_OPEN: &str = r#"{"number":7,"title":"T","state":"open","html_url":"http://gh/acme/app/pull/7","merged":false,"merged_at":null,"mergeable":true,"mergeable_state":"clean","head":{"ref":"pi/x","sha":"abc123"},"base":{"ref":"main","sha":"def"}}"#;
    const PULL_MERGED: &str = r#"{"number":7,"title":"T","state":"closed","html_url":"http://gh/acme/app/pull/7","merged":true,"merged_at":"2026-01-01T00:00:00Z","mergeable":null,"mergeable_state":"unknown","head":{"ref":"pi/x","sha":"abc123"},"base":{"ref":"main","sha":"def"}}"#;
    const LIST_ITEM_CLOSED: &str = r#"{"number":5,"title":"Old","state":"closed","html_url":"http://gh/acme/app/pull/5","head":{"ref":"pi/x","sha":"s5"},"base":{"ref":"main","sha":"d"}}"#;
    const LIST_ITEM_OPEN: &str = r#"{"number":7,"title":"T","state":"open","html_url":"http://gh/acme/app/pull/7","head":{"ref":"pi/x","sha":"abc123"},"base":{"ref":"main","sha":"d"}}"#;

    const PULLS: &str = "/api/v3/repos/acme/app/pulls";

    fn assert_github_headers(recorded: &Recorded) {
        assert_eq!(recorded.authorization.as_deref(), Some("Bearer ghp_test"));
        assert_eq!(recorded.user_agent.as_deref(), Some("iworkflow"));
        assert_eq!(
            recorded.accept.as_deref(),
            Some("application/vnd.github+json")
        );
        assert_eq!(recorded.api_version.as_deref(), Some("2022-11-28"));
    }

    #[test]
    fn pull_state_distinguishes_merged_from_closed() {
        assert_eq!(map_pull_state("open", false), "opened");
        assert_eq!(map_pull_state("closed", true), "merged");
        assert_eq!(map_pull_state("closed", false), "closed");
    }

    #[test]
    fn merge_status_is_normalised() {
        for clean in ["clean", "behind", "unstable", "has_hooks"] {
            assert_eq!(map_merge_status(Some(clean)), "mergeable");
        }
        assert_eq!(map_merge_status(Some("dirty")), "conflict");
        assert_eq!(map_merge_status(Some("blocked")), "blocked_status");
        assert_eq!(map_merge_status(Some("draft")), "not_approved");
        assert_eq!(map_merge_status(Some("unknown")), "checking");
        assert_eq!(map_merge_status(None), "checking");
    }

    #[test]
    fn ci_status_is_normalised() {
        for pending in ["queued", "waiting", "pending", "requested"] {
            assert_eq!(map_ci_status(pending, None), "pending");
        }
        assert_eq!(map_ci_status("in_progress", None), "running");
        let completed: [(&str, &str); 8] = [
            ("success", "success"),
            ("failure", "failed"),
            ("timed_out", "failed"),
            ("startup_failure", "failed"),
            ("cancelled", "canceled"),
            ("skipped", "skipped"),
            ("neutral", "skipped"),
            ("action_required", "manual"),
        ];
        for (conclusion, expected) in completed {
            assert_eq!(map_ci_status("completed", Some(conclusion)), expected);
        }
    }

    #[test]
    fn paths_encode_segments_and_keep_slashes() {
        assert_eq!(repo_path("acme/app"), "/repos/acme/app");
        assert_eq!(repo_path("a b/c#d"), "/repos/a%20b/c%23d");
        assert_eq!(ref_path("pi/fe at"), "pi/fe%20at");
    }

    #[test]
    fn merged_at_alone_marks_the_pull_as_merged() {
        let pull: PullJson = serde_json::from_str(
            r#"{"number":1,"title":"T","state":"closed","html_url":"u","merged_at":"2026-01-01","head":{"ref":"a","sha":"s"},"base":{"ref":"b","sha":"s"}}"#,
        )
        .expect("json");

        let mr: MergeRequest = pull.into();

        assert_eq!(mr.state, "merged");
        assert_eq!(mr.merge_status, "checking");
    }

    #[tokio::test]
    async fn current_user_sends_github_headers_and_returns_login() {
        let (settings, seen) =
            fake_github(vec![("GET /api/v3/user", 200, r#"{"login":"pablo"}"#)]).await;

        let user: String = GitHubHttpClientAdapter::new()
            .current_user(&settings)
            .await
            .expect("user");

        assert_eq!(user, "pablo");
        assert_github_headers(&seen.lock().expect("lock")[0]);
    }

    #[tokio::test]
    async fn branch_exists_maps_404_to_false() {
        let (settings, seen) = fake_github(vec![(
            "GET /api/v3/repos/acme/app/git/ref/heads/pi/x",
            200,
            r#"{"ref":"refs/heads/pi/x"}"#,
        )])
        .await;
        let client: GitHubHttpClientAdapter = GitHubHttpClientAdapter::new();

        assert!(
            client
                .branch_exists(&settings, "acme/app", "pi/x")
                .await
                .expect("ok")
        );
        assert!(
            !client
                .branch_exists(&settings, "acme/app", "otra")
                .await
                .expect("ok")
        );
        assert_eq!(seen.lock().expect("lock").len(), 2);
    }

    #[tokio::test]
    async fn find_latest_prefers_the_open_pull_and_rereads_it() {
        let list: String = format!("[{LIST_ITEM_CLOSED},{LIST_ITEM_OPEN}]");
        let list_route: String = format!(
            "GET {PULLS}?head=acme%3Api%2Fx&state=all&sort=created&direction=desc&per_page=30"
        );
        let (settings, seen) = fake_github(vec![
            (list_route.as_str(), 200, list.as_str()),
            ("GET /api/v3/repos/acme/app/pulls/7", 200, PULL_OPEN),
        ])
        .await;

        let mr: MergeRequest = GitHubHttpClientAdapter::new()
            .find_latest_merge_request(&settings, "acme/app", "pi/x")
            .await
            .expect("find")
            .expect("some");

        assert_eq!((mr.iid, mr.state.as_str()), (7, "opened"));
        assert_eq!(mr.merge_status, "mergeable");
        assert_eq!(mr.web_url, "http://gh/acme/app/pull/7");
        assert_eq!(
            (mr.source_branch.as_str(), mr.target_branch.as_str()),
            ("pi/x", "main")
        );
        let seen = seen.lock().expect("lock");
        assert_eq!(seen.len(), 2);
        assert_github_headers(&seen[0]);
        assert_github_headers(&seen[1]);
    }

    #[tokio::test]
    async fn find_latest_falls_back_to_most_recent_and_none_when_empty() {
        let closed_list: String = format!("[{LIST_ITEM_CLOSED}]");
        let closed_route: String = format!(
            "GET {PULLS}?head=acme%3Acerrada&state=all&sort=created&direction=desc&per_page=30"
        );
        let empty_route: String = format!(
            "GET {PULLS}?head=acme%3Avacia&state=all&sort=created&direction=desc&per_page=30"
        );
        let (settings, _) = fake_github(vec![
            (closed_route.as_str(), 200, closed_list.as_str()),
            (empty_route.as_str(), 200, "[]"),
            ("GET /api/v3/repos/acme/app/pulls/5", 200, PULL_MERGED),
        ])
        .await;
        let client: GitHubHttpClientAdapter = GitHubHttpClientAdapter::new();

        let found: Option<MergeRequest> = client
            .find_latest_merge_request(&settings, "acme/app", "cerrada")
            .await
            .expect("find");
        let missing: Option<MergeRequest> = client
            .find_latest_merge_request(&settings, "acme/app", "vacia")
            .await
            .expect("find");

        assert_eq!(
            found.map(|mr: MergeRequest| mr.state),
            Some("merged".to_string())
        );
        assert_eq!(missing, None);
    }

    #[tokio::test]
    async fn create_merge_request_posts_expected_body() {
        let (settings, seen) =
            fake_github(vec![("POST /api/v3/repos/acme/app/pulls", 201, PULL_OPEN)]).await;

        let created: MergeRequest = GitHubHttpClientAdapter::new()
            .create_merge_request(
                &settings,
                "acme/app",
                &NewMergeRequest {
                    source_branch: "pi/x".to_string(),
                    target_branch: "main".to_string(),
                    title: "T".to_string(),
                    description: "D".to_string(),
                },
            )
            .await
            .expect("create");

        assert_eq!(created.iid, 7);
        let seen = seen.lock().expect("lock");
        assert_github_headers(&seen[0]);
        let body: Value = serde_json::from_str(&seen[0].body).expect("json");
        assert_eq!(
            body,
            json!({"title":"T","head":"pi/x","base":"main","body":"D"})
        );
    }

    #[tokio::test]
    async fn create_merge_request_without_commits_is_a_clear_error() {
        let (settings, _) = fake_github(vec![(
            "POST /api/v3/repos/acme/app/pulls",
            422,
            r#"{"message":"Validation Failed","errors":[{"resource":"PullRequest","code":"custom","message":"No commits between main and pi/x"}]}"#,
        )])
        .await;

        let result = GitHubHttpClientAdapter::new()
            .create_merge_request(
                &settings,
                "acme/app",
                &NewMergeRequest {
                    source_branch: "pi/x".to_string(),
                    target_branch: "main".to_string(),
                    title: "T".to_string(),
                    description: "D".to_string(),
                },
            )
            .await;

        assert!(matches!(
            result,
            Err(CodeHostError::NoCommitsToPropose { ref branch, ref base })
                if branch == "pi/x" && base == "main"
        ));
    }

    #[tokio::test]
    async fn get_merge_request_maps_a_merged_pull() {
        let (settings, _) = fake_github(vec![(
            "GET /api/v3/repos/acme/app/pulls/7",
            200,
            PULL_MERGED,
        )])
        .await;

        let mr: MergeRequest = GitHubHttpClientAdapter::new()
            .get_merge_request(&settings, "acme/app", 7)
            .await
            .expect("get");

        assert_eq!(mr.state, "merged");
        assert!(!mr.is_open());
    }

    #[tokio::test]
    async fn merge_puts_then_rereads_the_pull() {
        let (settings, seen) = fake_github(vec![
            (
                "PUT /api/v3/repos/acme/app/pulls/7/merge",
                200,
                r#"{"merged":true,"sha":"abc","message":"Pull Request successfully merged"}"#,
            ),
            ("GET /api/v3/repos/acme/app/pulls/7", 200, PULL_MERGED),
        ])
        .await;

        let mr: MergeRequest = GitHubHttpClientAdapter::new()
            .merge_merge_request(&settings, "acme/app", 7)
            .await
            .expect("merge");

        assert_eq!(mr.state, "merged");
        let seen = seen.lock().expect("lock");
        assert_eq!(seen[0].method, "PUT");
        assert_eq!(seen[0].body, "{}");
        assert_eq!(seen[1].method, "GET");
    }

    #[tokio::test]
    async fn merge_refusal_surfaces_githubs_message() {
        let (settings, _) = fake_github(vec![(
            "PUT /api/v3/repos/acme/app/pulls/8/merge",
            405,
            r#"{"message":"Pull Request is not mergeable"}"#,
        )])
        .await;

        let err: CodeHostError = GitHubHttpClientAdapter::new()
            .merge_merge_request(&settings, "acme/app", 8)
            .await
            .unwrap_err();

        assert_eq!(
            err,
            CodeHostError::Api {
                provider: HostingProvider::GitHub,
                status: 405,
                message: "Pull Request is not mergeable".to_string()
            }
        );
    }

    #[tokio::test]
    async fn latest_pipeline_maps_run_and_jobs() {
        let (settings, _) = fake_github(vec![
            ("GET /api/v3/repos/acme/app/pulls/7", 200, PULL_OPEN),
            (
                "GET /api/v3/repos/acme/app/actions/runs?head_sha=abc123&per_page=1",
                200,
                r#"{"total_count":1,"workflow_runs":[{"id":22,"name":"CI","status":"completed","conclusion":"failure","html_url":"http://gh/runs/22"}]}"#,
            ),
            (
                "GET /api/v3/repos/acme/app/actions/runs/22/jobs?per_page=100",
                200,
                r#"{"total_count":2,"jobs":[{"id":1,"name":"build","status":"completed","conclusion":"success","html_url":"http://gh/j/1"},{"id":2,"name":"test","status":"in_progress","conclusion":null,"html_url":"http://gh/j/2"}]}"#,
            ),
        ])
        .await;

        let pipeline: Pipeline = GitHubHttpClientAdapter::new()
            .latest_pipeline(&settings, "acme/app", 7)
            .await
            .expect("pipeline")
            .expect("some");

        assert_eq!((pipeline.id, pipeline.status.as_str()), (22, "failed"));
        assert_eq!(pipeline.web_url, "http://gh/runs/22");
        assert_eq!(pipeline.jobs.len(), 2);
        assert_eq!(
            (
                pipeline.jobs[0].stage.as_str(),
                pipeline.jobs[0].status.as_str()
            ),
            ("CI", "success")
        );
        assert_eq!(pipeline.jobs[1].status, "running");
    }

    #[tokio::test]
    async fn latest_pipeline_is_none_without_runs() {
        let (settings, seen) = fake_github(vec![
            ("GET /api/v3/repos/acme/app/pulls/7", 200, PULL_OPEN),
            (
                "GET /api/v3/repos/acme/app/actions/runs?head_sha=abc123&per_page=1",
                200,
                r#"{"total_count":0,"workflow_runs":[]}"#,
            ),
        ])
        .await;

        let pipeline: Option<Pipeline> = GitHubHttpClientAdapter::new()
            .latest_pipeline(&settings, "acme/app", 7)
            .await
            .expect("pipeline");

        assert_eq!(pipeline, None);
        assert_eq!(seen.lock().expect("lock").len(), 2);
    }

    #[tokio::test]
    async fn unauthorized_reports_the_github_error() {
        let (settings, _) = fake_github(vec![(
            "GET /api/v3/user",
            401,
            r#"{"message":"Bad credentials"}"#,
        )])
        .await;

        let err: CodeHostError = GitHubHttpClientAdapter::new()
            .current_user(&settings)
            .await
            .unwrap_err();

        assert_eq!(
            err,
            CodeHostError::Api {
                provider: HostingProvider::GitHub,
                status: 401,
                message: "Bad credentials".to_string()
            }
        );
    }

    #[tokio::test]
    async fn network_failure_does_not_leak_token() {
        let settings: CodeHostSettings =
            CodeHostSettings::new(HostingProvider::GitHub, "http://127.0.0.1:1", TOKEN, false)
                .expect("settings");

        let err: CodeHostError = GitHubHttpClientAdapter::new()
            .current_user(&settings)
            .await
            .unwrap_err();

        assert!(matches!(err, CodeHostError::Network(_)));
        assert!(!err.to_string().contains(TOKEN));
    }

    #[tokio::test]
    async fn redirect_to_another_host_never_receives_the_token() {
        let leaked: Arc<Mutex<Vec<String>>> = Arc::new(Mutex::new(Vec::new()));
        let sink: Arc<Mutex<Vec<String>>> = leaked.clone();
        let target: Router =
            Router::new().fallback(axum::routing::get(move |headers: HeaderMap| {
                let sink: Arc<Mutex<Vec<String>>> = sink.clone();
                async move {
                    if let (Some(value), Ok(mut seen)) = (headers.get("authorization"), sink.lock())
                    {
                        seen.push(value.to_str().unwrap_or_default().to_string());
                    }
                    (HttpStatus::OK, r#"{"login":"intruso"}"#)
                }
            }));
        let target_listener: TcpListener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let target_addr: std::net::SocketAddr = target_listener.local_addr().expect("addr");
        tokio::spawn(async move {
            let _ = axum::serve(target_listener, target).await;
        });
        let location: String = format!("http://{target_addr}/api/v3/user");
        let origin: Router = Router::new().fallback(axum::routing::get(move || {
            let location: String = location.clone();
            async move { (HttpStatus::FOUND, [("location", location)], "") }
        }));
        let origin_listener: TcpListener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let origin_addr: std::net::SocketAddr = origin_listener.local_addr().expect("addr");
        tokio::spawn(async move {
            let _ = axum::serve(origin_listener, origin).await;
        });
        let settings: CodeHostSettings = CodeHostSettings::new(
            HostingProvider::GitHub,
            &format!("http://{origin_addr}"),
            "ghp_secret",
            false,
        )
        .expect("settings");

        let result: Result<String, CodeHostError> =
            GitHubHttpClientAdapter::new().current_user(&settings).await;

        assert_eq!(
            result,
            Err(CodeHostError::Redirected {
                provider: HostingProvider::GitHub,
                status: 302
            })
        );
        assert!(leaked.lock().expect("lock").is_empty());
    }
}
