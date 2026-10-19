//! Rejects cross-site writes. A page on another site can make a browser POST to this app
//! (server functions accept plain form bodies, so no CORS preflight happens); browsers always
//! label such requests with an `Origin` that differs from the `Host` they were sent to.

use axum::extract::Request;
use axum::http::header::{HOST, ORIGIN};
use axum::http::{HeaderMap, Method, StatusCode};
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};

const SAFE_METHODS: [Method; 3] = [Method::GET, Method::HEAD, Method::OPTIONS];

/// `Origin` for a page served over http(s) is `scheme://authority`; this keeps the authority.
fn origin_authority(origin: &str) -> Option<&str> {
    origin
        .strip_prefix("http://")
        .or_else(|| origin.strip_prefix("https://"))
}

/// Whether a request is a write that a browser sent from a different origin than the host
/// it targets. Requests without `Origin` (curl, native clients) are not browser cross-site.
pub fn is_cross_site_write(method: &Method, headers: &HeaderMap) -> bool {
    if SAFE_METHODS.contains(method) {
        return false;
    }
    let Some(origin) = headers.get(ORIGIN).and_then(|v| v.to_str().ok()) else {
        return false;
    };
    let host: Option<&str> = headers.get(HOST).and_then(|v| v.to_str().ok());
    match (origin_authority(origin), host) {
        (Some(authority), Some(host)) => !authority.eq_ignore_ascii_case(host),
        // `null`, unknown schemes or no Host: cannot be proven same-origin.
        _ => true,
    }
}

/// Axum middleware answering 403 to cross-site writes.
pub async fn reject_cross_site_writes(request: Request, next: Next) -> Response {
    if is_cross_site_write(request.method(), request.headers()) {
        return StatusCode::FORBIDDEN.into_response();
    }
    next.run(request).await
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::http::HeaderValue;

    fn headers(origin: Option<&str>, host: Option<&str>) -> HeaderMap {
        let mut map: HeaderMap = HeaderMap::new();
        if let Some(origin) = origin {
            map.insert(ORIGIN, HeaderValue::from_str(origin).expect("origin"));
        }
        if let Some(host) = host {
            map.insert(HOST, HeaderValue::from_str(host).expect("host"));
        }
        map
    }

    #[test]
    fn same_origin_write_is_allowed() {
        let map: HeaderMap = headers(Some("http://192.168.1.5:3000"), Some("192.168.1.5:3000"));
        assert!(!is_cross_site_write(&Method::POST, &map));
    }

    #[test]
    fn other_origin_write_is_rejected() {
        let map: HeaderMap = headers(Some("https://evil.example"), Some("localhost:3000"));
        assert!(is_cross_site_write(&Method::POST, &map));
        assert!(is_cross_site_write(&Method::DELETE, &map));
    }

    #[test]
    fn opaque_origin_is_rejected() {
        let map: HeaderMap = headers(Some("null"), Some("localhost:3000"));
        assert!(is_cross_site_write(&Method::POST, &map));
    }

    #[test]
    fn reads_and_originless_clients_are_not_affected() {
        let cross: HeaderMap = headers(Some("https://evil.example"), Some("localhost:3000"));
        assert!(!is_cross_site_write(&Method::GET, &cross));
        let curl: HeaderMap = headers(None, Some("localhost:3000"));
        assert!(!is_cross_site_write(&Method::POST, &curl));
    }
}
