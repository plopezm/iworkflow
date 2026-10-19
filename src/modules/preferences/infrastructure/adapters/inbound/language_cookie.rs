//! Keeps the `iw-lang` cookie in step with the language stored in the database, so a browser
//! that never picked a language (new browser, cleared cookies) opens in the stored one instead of
//! asking again. The cookie is rewritten on page loads, before SSR reads it.

use axum::extract::{Request, State};
use axum::http::header::{ACCEPT, COOKIE, SET_COOKIE};
use axum::http::{HeaderValue, Method};
use axum::middleware::Next;
use axum::response::Response;

use crate::core::i18n::{COOKIE_MAX_AGE_SECS, LOCALE_COOKIE, Locale};
use crate::modules::preferences::domain::models::ui_language::UiLanguage;
use crate::modules::preferences::infrastructure::di::preferences_context::PreferencesContext;

/// What to do with the language of a page request.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Sync {
    /// Cookie and database already agree (or neither knows a language).
    Keep,
    /// The database wins: this browser gets its language.
    Restore(&'static str),
    /// Only the cookie knows the language (chosen before it was stored): save it.
    Persist(&'static str),
}

fn reconcile(stored: Option<&'static str>, cookie: Option<Locale>) -> Sync {
    match (stored, cookie) {
        (Some(stored), Some(cookie)) if cookie.code() == stored => Sync::Keep,
        (Some(stored), _) => Sync::Restore(stored),
        (None, Some(cookie)) => Sync::Persist(cookie.code()),
        (None, None) => Sync::Keep,
    }
}

/// The `Cookie` header value with the language cookie replaced by `code`.
fn cookie_header_with_language(header: Option<&str>, code: &str) -> String {
    header
        .unwrap_or_default()
        .split(';')
        .map(str::trim)
        .filter(|pair: &&str| !pair.is_empty() && !is_language_pair(pair))
        .map(str::to_string)
        .chain(std::iter::once(format!("{LOCALE_COOKIE}={code}")))
        .collect::<Vec<String>>()
        .join("; ")
}

fn is_language_pair(pair: &str) -> bool {
    pair.split_once('=')
        .is_some_and(|(name, _): (&str, &str)| name.trim() == LOCALE_COOKIE)
}

/// Whether the request loads a page (the only ones that render in a language).
fn is_page_request(request: &Request) -> bool {
    request.method() == Method::GET
        && request
            .headers()
            .get(ACCEPT)
            .and_then(|value: &HeaderValue| value.to_str().ok())
            .is_some_and(|accept: &str| accept.contains("text/html"))
}

/// Axum middleware: applies [`reconcile`] on page requests. Storage failures are logged and the
/// request proceeds with whatever the cookie says.
pub async fn sync_language_cookie(
    State(preferences): State<PreferencesContext>,
    mut request: Request,
    next: Next,
) -> Response {
    if !is_page_request(&request) {
        return next.run(request).await;
    }
    let header: Option<String> = request
        .headers()
        .get(COOKIE)
        .and_then(|value: &HeaderValue| value.to_str().ok())
        .map(str::to_string);
    let cookie: Option<Locale> = header.as_deref().and_then(Locale::from_cookie_header);
    let stored: Option<UiLanguage> = match preferences.get_language.execute().await {
        Ok(stored) => stored,
        Err(error) => {
            tracing::warn!(%error, "no se pudo leer el idioma guardado");
            return next.run(request).await;
        }
    };
    let mut set_cookie: Option<&'static str> = None;
    match reconcile(stored.as_ref().map(UiLanguage::as_str), cookie) {
        Sync::Keep => {}
        Sync::Restore(code) => {
            let rewritten: String = cookie_header_with_language(header.as_deref(), code);
            if let Ok(value) = HeaderValue::from_str(&rewritten) {
                request.headers_mut().insert(COOKIE, value);
                set_cookie = Some(code);
            }
        }
        Sync::Persist(code) => {
            if let Err(error) = preferences.save_language.execute(code).await {
                tracing::warn!(%error, "no se pudo guardar el idioma elegido");
            }
        }
    }
    let mut response: Response = next.run(request).await;
    if let Some(code) = set_cookie
        && let Ok(value) = HeaderValue::from_str(&format!(
            "{LOCALE_COOKIE}={code}; Path=/; Max-Age={COOKIE_MAX_AGE_SECS}; SameSite=Lax"
        ))
    {
        response.headers_mut().append(SET_COOKIE, value);
    }
    response
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn database_language_wins_over_a_missing_or_different_cookie() {
        assert_eq!(reconcile(Some("de"), None), Sync::Restore("de"));
        assert_eq!(reconcile(Some("de"), Some(Locale::Es)), Sync::Restore("de"));
    }

    #[test]
    fn matching_cookie_and_database_need_nothing() {
        assert_eq!(reconcile(Some("de"), Some(Locale::De)), Sync::Keep);
        assert_eq!(reconcile(None, None), Sync::Keep);
    }

    #[test]
    fn a_cookie_chosen_before_storage_is_persisted() {
        assert_eq!(reconcile(None, Some(Locale::Fr)), Sync::Persist("fr"));
    }

    #[test]
    fn cookie_header_replaces_only_the_language_pair() {
        assert_eq!(
            cookie_header_with_language(Some("a=1; iw-lang=es; b=2"), "de"),
            "a=1; b=2; iw-lang=de"
        );
        assert_eq!(cookie_header_with_language(None, "fr"), "iw-lang=fr");
        assert_eq!(
            cookie_header_with_language(Some("other-iw-lang=x"), "zh"),
            "other-iw-lang=x; iw-lang=zh"
        );
    }

    #[test]
    fn rewritten_header_is_read_back_as_the_stored_language() {
        let header: String = cookie_header_with_language(Some("iw-lang=es; a=1"), "zh");
        assert_eq!(Locale::from_cookie_header(&header), Some(Locale::Zh));
    }
}
