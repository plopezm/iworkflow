//! Optional access-token guard for when the app is reachable from the network. When enforcing,
//! every request must carry the current token (bearer header, cookie or one-shot `?token=` link,
//! e.g. from a QR code); anything else gets a 401, with a small login page for browsers.

use std::sync::{Arc, PoisonError, RwLock};

use axum::Router;
use axum::extract::{DefaultBodyLimit, Form, Request, State};
use axum::http::header::SET_COOKIE;
use axum::http::header::{ACCEPT, AUTHORIZATION, CACHE_CONTROL, CONTENT_TYPE, COOKIE, LOCATION};
use axum::http::{HeaderMap, HeaderValue, Method, StatusCode, Uri};
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};
use axum::routing::post;
use serde::Deserialize;

use crate::core::i18n::{Locale, Msg};
use crate::modules::preferences::domain::models::access_token::AccessToken;

/// Name of the cookie that keeps the token in the browser.
pub const ACCESS_COOKIE: &str = "iw-token";
/// Route that receives the login form.
pub const LOGIN_PATH: &str = "/access-login";

/// Query parameter that carries the token in shareable links.
const TOKEN_PARAM: &str = "token";
const BEARER_PREFIX: &str = "Bearer ";
const COOKIE_MAX_AGE_SECS: u32 = 31_536_000;
/// The login form only holds a 64-character token.
const LOGIN_BODY_LIMIT_BYTES: usize = 4096;
const NO_STORE: &str = "no-store";

const LOGIN_TITLE: Msg = Msg {
    en: "Access token required",
    es: "Se requiere un token de acceso",
    zh: "需要访问令牌",
    de: "Zugriffstoken erforderlich",
    fr: "Jeton d'accès requis",
};

const LOGIN_LABEL: Msg = Msg {
    en: "Access token",
    es: "Token de acceso",
    zh: "访问令牌",
    de: "Zugriffstoken",
    fr: "Jeton d'accès",
};

const LOGIN_SUBMIT: Msg = Msg {
    en: "Sign in",
    es: "Entrar",
    zh: "登录",
    de: "Anmelden",
    fr: "Se connecter",
};

const LOGIN_INVALID: Msg = Msg {
    en: "Invalid token.",
    es: "Token no válido.",
    zh: "令牌无效。",
    de: "Ungültiges Token.",
    fr: "Jeton invalide.",
};

/// Shared guard state: the current token (replaceable at runtime) and whether it is enforced.
#[derive(Clone)]
pub struct AccessGuard {
    token: Arc<RwLock<AccessToken>>,
    enforce: bool,
}

impl AccessGuard {
    pub fn new(token: AccessToken, enforce: bool) -> Self {
        Self {
            token: Arc::new(RwLock::new(token)),
            enforce,
        }
    }

    /// Replaces the token; takes effect for the next request.
    pub fn set_token(&self, token: AccessToken) {
        let mut current = self.token.write().unwrap_or_else(PoisonError::into_inner);
        *current = token;
    }

    pub fn is_enforcing(&self) -> bool {
        self.enforce
    }

    /// Value for a `Set-Cookie` header that stores the current token in the browser.
    pub fn session_cookie(&self) -> String {
        let token: AccessToken = self.current();
        format!(
            "{ACCESS_COOKIE}={}; HttpOnly; SameSite=Strict; Path=/; Max-Age={COOKIE_MAX_AGE_SECS}",
            token.as_str()
        )
    }

    fn current(&self) -> AccessToken {
        self.token
            .read()
            .unwrap_or_else(PoisonError::into_inner)
            .clone()
    }

    fn accepts(&self, candidate: &str) -> bool {
        self.token
            .read()
            .unwrap_or_else(PoisonError::into_inner)
            .matches(candidate)
    }
}

/// Where a presented token was found.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Source {
    Bearer,
    Cookie,
    Query,
}

/// The `Authorization: Bearer` value, if any.
fn bearer_token(headers: &HeaderMap) -> Option<String> {
    headers
        .get(AUTHORIZATION)
        .and_then(|value: &HeaderValue| value.to_str().ok())
        .and_then(|value: &str| value.strip_prefix(BEARER_PREFIX))
        .map(|value: &str| value.trim().to_string())
}

/// The value of cookie `name` across all `Cookie` headers.
fn cookie_value(headers: &HeaderMap, name: &str) -> Option<String> {
    headers
        .get_all(COOKIE)
        .iter()
        .filter_map(|value: &HeaderValue| value.to_str().ok())
        .flat_map(|header: &str| header.split(';'))
        .filter_map(|pair: &str| pair.trim().split_once('='))
        .find(|(key, _): &(&str, &str)| key.trim() == name)
        .map(|(_, value): (&str, &str)| value.trim().to_string())
}

/// The value of the `token` query parameter, if any.
fn query_token(query: Option<&str>) -> Option<String> {
    query?
        .split('&')
        .filter_map(|pair: &str| pair.split_once('='))
        .find(|(key, _): &(&str, &str)| *key == TOKEN_PARAM)
        .map(|(_, value): (&str, &str)| value.to_string())
}

/// The query string without the `token` parameter (`None` when nothing remains).
fn strip_token_param(query: Option<&str>) -> Option<String> {
    let remaining: Vec<&str> = query?
        .split('&')
        .filter(|pair: &&str| !pair.is_empty() && pair.split('=').next() != Some(TOKEN_PARAM))
        .collect();
    if remaining.is_empty() {
        None
    } else {
        Some(remaining.join("&"))
    }
}

/// Same path as `uri` without the token parameter, ready for a `Location` header.
fn clean_location(uri: &Uri) -> String {
    // Collapse leading slashes: `//host/` would be read by browsers as another origin.
    let path: String = format!("/{}", uri.path().trim_start_matches('/'));
    match strip_token_param(uri.query()) {
        Some(rest) => format!("{path}?{rest}"),
        None => path,
    }
}

/// Tokens the request presents, in priority order.
fn presented_tokens(headers: &HeaderMap, uri: &Uri) -> Vec<(Source, String)> {
    let mut found: Vec<(Source, String)> = Vec::new();
    if let Some(token) = bearer_token(headers) {
        found.push((Source::Bearer, token));
    }
    if let Some(token) = cookie_value(headers, ACCESS_COOKIE) {
        found.push((Source::Cookie, token));
    }
    if let Some(token) = query_token(uri.query()) {
        found.push((Source::Query, token));
    }
    found
}

fn locale_of(headers: &HeaderMap) -> Locale {
    headers
        .get(COOKIE)
        .and_then(|value: &HeaderValue| value.to_str().ok())
        .and_then(Locale::from_cookie_header)
        .unwrap_or(Locale::En)
}

fn wants_html(request: &Request) -> bool {
    request.method() == Method::GET
        && request
            .headers()
            .get(ACCEPT)
            .and_then(|value: &HeaderValue| value.to_str().ok())
            .is_some_and(|accept: &str| accept.contains("text/html"))
}

fn login_page(locale: Locale, invalid: bool) -> String {
    let error: String = if invalid {
        format!(
            "<p class=\"error\" role=\"alert\">{}</p>",
            LOGIN_INVALID.in_locale(locale)
        )
    } else {
        String::new()
    };
    format!(
        "<!doctype html><html lang=\"{lang}\"><head><meta charset=\"utf-8\">\
<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\
<meta name=\"robots\" content=\"noindex\"><title>{title}</title><style>\
body{{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;\
background:#0b0d12;color:#e6e8ee;font:16px system-ui,sans-serif}}\
form{{width:min(22rem,90vw);display:flex;flex-direction:column;gap:.75rem;padding:1.5rem;\
border:1px solid #2a2f3a;border-radius:1rem;background:#141821}}\
h1{{margin:0;font-size:1.1rem}}label{{font-size:.85rem;color:#9aa3b5}}\
input{{padding:.6rem .7rem;border-radius:.6rem;border:1px solid #2a2f3a;background:#0b0d12;\
color:inherit;font:inherit}}\
button{{padding:.6rem;border:0;border-radius:.6rem;background:#4f7cff;color:#fff;font:inherit;\
cursor:pointer}}.error{{margin:0;color:#ff7b7b;font-size:.85rem}}\
</style></head><body><form method=\"post\" action=\"{action}\"><h1>{title}</h1>{error}\
<label for=\"token\">{label}</label>\
<input id=\"token\" name=\"token\" type=\"password\" autocomplete=\"current-password\" \
autofocus required><button type=\"submit\">{submit}</button></form></body></html>",
        lang = locale.code(),
        title = LOGIN_TITLE.in_locale(locale),
        action = LOGIN_PATH,
        label = LOGIN_LABEL.in_locale(locale),
        submit = LOGIN_SUBMIT.in_locale(locale),
    )
}

fn with_no_store(mut response: Response) -> Response {
    response
        .headers_mut()
        .insert(CACHE_CONTROL, HeaderValue::from_static(NO_STORE));
    response
}

fn unauthorized(html: Option<(Locale, bool)>) -> Response {
    let response: Response = match html {
        Some((locale, invalid)) => (
            StatusCode::UNAUTHORIZED,
            [(CONTENT_TYPE, "text/html; charset=utf-8")],
            login_page(locale, invalid),
        )
            .into_response(),
        None => StatusCode::UNAUTHORIZED.into_response(),
    };
    with_no_store(response)
}

/// `303 See Other` to `location`, storing the current token in the browser.
fn redirect_with_session(guard: &AccessGuard, location: &str) -> Response {
    let mut response: Response = StatusCode::SEE_OTHER.into_response();
    if let Ok(value) = HeaderValue::from_str(location) {
        response.headers_mut().insert(LOCATION, value);
    }
    if let Ok(value) = HeaderValue::from_str(&guard.session_cookie()) {
        response.headers_mut().insert(SET_COOKIE, value);
    }
    with_no_store(response)
}

/// Axum middleware: lets the request through when the guard is off or the request carries the
/// current token; otherwise answers 401. The login route is skipped (it checks the token itself).
pub async fn require_access(
    State(guard): State<AccessGuard>,
    request: Request,
    next: Next,
) -> Response {
    // Only the login submission skips the check: any other method on that path (which the
    // application fallback could answer) still needs a token.
    let is_login: bool = request.uri().path() == LOGIN_PATH && request.method() == Method::POST;
    if !guard.is_enforcing() || is_login {
        return next.run(request).await;
    }
    let accepted: Option<Source> = presented_tokens(request.headers(), request.uri())
        .into_iter()
        .find(|(_, token): &(Source, String)| guard.accepts(token))
        .map(|(source, _): (Source, String)| source);
    match accepted {
        Some(Source::Query) if matches!(*request.method(), Method::GET | Method::HEAD) => {
            redirect_with_session(&guard, &clean_location(request.uri()))
        }
        Some(_) => next.run(request).await,
        None => {
            let html: Option<(Locale, bool)> =
                wants_html(&request).then(|| (locale_of(request.headers()), false));
            unauthorized(html)
        }
    }
}

#[derive(Deserialize)]
struct LoginForm {
    #[serde(default)]
    token: String,
}

async fn login(
    State(guard): State<AccessGuard>,
    headers: HeaderMap,
    Form(form): Form<LoginForm>,
) -> Response {
    if !guard.is_enforcing() || guard.accepts(&form.token) {
        return redirect_with_session_if_enforcing(&guard);
    }
    unauthorized(Some((locale_of(&headers), true)))
}

/// Redirects to the home page; the cookie is only set when there is a token to protect.
fn redirect_with_session_if_enforcing(guard: &AccessGuard) -> Response {
    if guard.is_enforcing() {
        redirect_with_session(guard, "/")
    } else {
        with_no_store((StatusCode::SEE_OTHER, [(LOCATION, "/")]).into_response())
    }
}

/// `POST /access-login`, already bound to `guard`.
pub fn access_router(guard: AccessGuard) -> Router {
    Router::new()
        .route(
            LOGIN_PATH,
            post(login).layer(DefaultBodyLimit::max(LOGIN_BODY_LIMIT_BYTES)),
        )
        .with_state(guard)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn headers_with(name: axum::http::HeaderName, value: &str) -> HeaderMap {
        let mut headers: HeaderMap = HeaderMap::new();
        if let Ok(value) = HeaderValue::from_str(value) {
            headers.insert(name, value);
        }
        headers
    }

    #[test]
    fn cookie_is_found_among_several() {
        let headers: HeaderMap = headers_with(COOKIE, "a=1; iw-token=abc; iw-lang=es");
        assert_eq!(
            cookie_value(&headers, ACCESS_COOKIE).as_deref(),
            Some("abc")
        );
        assert_eq!(cookie_value(&headers, "missing"), None);
    }

    #[test]
    fn cookie_name_must_match_exactly() {
        let headers: HeaderMap = headers_with(COOKIE, "x-iw-token=abc");
        assert_eq!(cookie_value(&headers, ACCESS_COOKIE), None);
    }

    #[test]
    fn bearer_requires_the_prefix() {
        assert_eq!(
            bearer_token(&headers_with(AUTHORIZATION, "Bearer abc")).as_deref(),
            Some("abc")
        );
        assert_eq!(
            bearer_token(&headers_with(AUTHORIZATION, "Basic abc")),
            None
        );
        assert_eq!(bearer_token(&HeaderMap::new()), None);
    }

    #[test]
    fn query_token_is_read_and_stripped() {
        assert_eq!(query_token(Some("a=1&token=xyz")).as_deref(), Some("xyz"));
        assert_eq!(query_token(Some("a=1")), None);
        assert_eq!(query_token(None), None);
        assert_eq!(
            strip_token_param(Some("a=1&token=xyz&b=2")).as_deref(),
            Some("a=1&b=2")
        );
        assert_eq!(strip_token_param(Some("token=xyz")), None);
        assert_eq!(
            strip_token_param(Some("mytoken=1&token=x")).as_deref(),
            Some("mytoken=1")
        );
    }

    #[test]
    fn location_drops_only_the_token() {
        let with_rest: Uri = "/tasks?x=1&token=abc".parse().unwrap_or_default();
        assert_eq!(clean_location(&with_rest), "/tasks?x=1");
        let alone: Uri = "/tasks?token=abc".parse().unwrap_or_default();
        assert_eq!(clean_location(&alone), "/tasks");
    }

    #[test]
    fn presented_tokens_follow_priority_order() {
        let mut headers: HeaderMap = headers_with(AUTHORIZATION, "Bearer b");
        headers.insert(COOKIE, HeaderValue::from_static("iw-token=c"));
        let uri: Uri = "/?token=q".parse().unwrap_or_default();
        let found: Vec<(Source, String)> = presented_tokens(&headers, &uri);
        let sources: Vec<Source> = found.iter().map(|(s, _): &(Source, String)| *s).collect();
        assert_eq!(sources, vec![Source::Bearer, Source::Cookie, Source::Query]);
    }

    #[test]
    fn login_page_is_localized_and_flags_errors() {
        let page: String = login_page(Locale::De, true);
        assert!(page.contains("lang=\"de\""));
        assert!(page.contains(LOGIN_INVALID.in_locale(Locale::De)));
        assert!(page.contains("action=\"/access-login\""));
        assert!(!login_page(Locale::En, false).contains("role=\"alert\""));
    }

    #[test]
    fn session_cookie_uses_the_current_token() {
        let first: AccessToken = AccessToken::generate();
        let guard: AccessGuard = AccessGuard::new(first.clone(), true);
        assert!(
            guard
                .session_cookie()
                .starts_with(&format!("iw-token={}", first.as_str()))
        );
        let second: AccessToken = AccessToken::generate();
        guard.set_token(second.clone());
        assert!(guard.session_cookie().contains(second.as_str()));
        assert!(!guard.accepts(first.as_str()));
    }

    #[test]
    fn clean_location_never_starts_with_two_slashes() {
        let uri: Uri = "//evil.example/x?token=abc&y=1".parse().expect("uri");
        assert_eq!(clean_location(&uri), "/evil.example/x?y=1");
        let root: Uri = "/?token=abc".parse().expect("uri");
        assert_eq!(clean_location(&root), "/");
    }
}
