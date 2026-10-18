//! Internationalisation: supported locales, compile-time-complete messages and detection.
//!
//! Every user-facing text is a [`Msg`] constant holding one string per language, so a missing
//! translation is a compile error. The chosen language lives in the `iw-lang` cookie, which both
//! the server (SSR) and the browser (hydration) can read synchronously; changing it stores the
//! cookie and reloads the page, so messages never need to be reactive.

use leptos::prelude::*;

use crate::core::browser;

/// Name of the cookie that stores the chosen language.
pub const LOCALE_COOKIE: &str = "iw-lang";

/// One year, in seconds.
const COOKIE_MAX_AGE_SECS: u32 = 31_536_000;

/// Languages the interface is available in.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Locale {
    #[default]
    En,
    Es,
    Zh,
    De,
    Fr,
}

impl Locale {
    /// Every supported language, in the order the selector lists them.
    pub const ALL: [Locale; 5] = [Locale::En, Locale::Es, Locale::Zh, Locale::De, Locale::Fr];

    /// ISO 639-1 code; also the cookie value and the `<html lang>` attribute.
    pub fn code(self) -> &'static str {
        match self {
            Locale::En => "en",
            Locale::Es => "es",
            Locale::Zh => "zh",
            Locale::De => "de",
            Locale::Fr => "fr",
        }
    }

    /// Name of the language written in that language.
    pub fn native_name(self) -> &'static str {
        match self {
            Locale::En => "English",
            Locale::Es => "Español",
            Locale::Zh => "中文",
            Locale::De => "Deutsch",
            Locale::Fr => "Français",
        }
    }

    /// Parses a language code; unknown codes yield `None`.
    pub fn parse(value: &str) -> Option<Locale> {
        Locale::ALL
            .into_iter()
            .find(|locale: &Locale| locale.code() == value.trim())
    }

    /// Extracts the chosen language from a `Cookie` header value.
    pub fn from_cookie_header(header: &str) -> Option<Locale> {
        header.split(';').find_map(|pair: &str| {
            let (name, value) = pair.split_once('=')?;
            (name.trim() == LOCALE_COOKIE)
                .then(|| Locale::parse(value))
                .flatten()
        })
    }
}

/// A user-facing text in every supported language.
///
/// Placeholders are written `{name}` and filled with [`Msg::fmt`].
#[derive(Debug, Clone, Copy)]
pub struct Msg {
    pub en: &'static str,
    pub es: &'static str,
    pub zh: &'static str,
    pub de: &'static str,
    pub fr: &'static str,
}

impl Msg {
    /// The text in a given language.
    pub fn in_locale(&self, locale: Locale) -> &'static str {
        match locale {
            Locale::En => self.en,
            Locale::Es => self.es,
            Locale::Zh => self.zh,
            Locale::De => self.de,
            Locale::Fr => self.fr,
        }
    }

    /// The text in the language of the current request or browser.
    pub fn get(&self) -> &'static str {
        self.in_locale(current_locale())
    }

    /// The current-language text with every `{name}` replaced by its value.
    pub fn fmt(&self, args: &[(&str, &str)]) -> String {
        args.iter().fold(
            self.get().to_string(),
            |text: String, (name, value): &(&str, &str)| {
                text.replace(&format!("{{{name}}}"), value)
            },
        )
    }
}

/// Language in force plus whether the user ever picked one.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct LocaleState {
    pub locale: Locale,
    /// `false` until the user picks a language; drives the first-run prompt.
    pub chosen: bool,
}

#[cfg(feature = "hydrate")]
static CLIENT_LOCALE: std::sync::atomic::AtomicU8 = std::sync::atomic::AtomicU8::new(0);

/// The language to render with. Server renders read it from the request context; browser code
/// that runs outside the reactive owner (event handlers, async tasks) falls back to a static.
pub fn current_locale() -> Locale {
    if let Some(state) = use_context::<LocaleState>() {
        return state.locale;
    }
    client_locale()
}

#[cfg(feature = "hydrate")]
fn client_locale() -> Locale {
    let index: u8 = CLIENT_LOCALE.load(std::sync::atomic::Ordering::Relaxed);
    Locale::ALL
        .get(usize::from(index))
        .copied()
        .unwrap_or_default()
}

#[cfg(not(feature = "hydrate"))]
fn client_locale() -> Locale {
    Locale::default()
}

#[cfg(feature = "ssr")]
fn cookie_header() -> Option<String> {
    let parts: axum::http::request::Parts = use_context::<axum::http::request::Parts>()?;
    parts
        .headers
        .get(axum::http::header::COOKIE)
        .and_then(|value: &axum::http::HeaderValue| value.to_str().ok())
        .map(str::to_string)
}

#[cfg(all(feature = "hydrate", not(feature = "ssr")))]
fn cookie_header() -> Option<String> {
    browser::read_cookies()
}

#[cfg(not(any(feature = "ssr", feature = "hydrate")))]
fn cookie_header() -> Option<String> {
    None
}

/// Reads the language from the current request (SSR) or the browser (hydration).
pub fn detect_locale() -> LocaleState {
    match cookie_header()
        .as_deref()
        .and_then(Locale::from_cookie_header)
    {
        Some(locale) => LocaleState {
            locale,
            chosen: true,
        },
        None => LocaleState::default(),
    }
}

/// Detects the language and provides it to the whole tree. Call once at the root.
pub fn provide_locale() -> LocaleState {
    let state: LocaleState = detect_locale();
    #[cfg(feature = "hydrate")]
    if let Some(index) = Locale::ALL
        .iter()
        .position(|locale: &Locale| *locale == state.locale)
    {
        CLIENT_LOCALE.store(index as u8, std::sync::atomic::Ordering::Relaxed);
    }
    provide_context(state);
    state
}

/// Stores the user's choice and reloads so every text is rendered in the new language.
pub fn choose_locale(locale: Locale) {
    browser::set_cookie(LOCALE_COOKIE, locale.code(), COOKIE_MAX_AGE_SECS);
    browser::reload_page();
}

#[cfg(test)]
mod tests {
    use super::*;

    const GREETING: Msg = Msg {
        en: "Hello {name}",
        es: "Hola {name}",
        zh: "你好 {name}",
        de: "Hallo {name}",
        fr: "Bonjour {name}",
    };

    #[test]
    fn parse_accepts_every_supported_code_and_rejects_others() {
        for locale in Locale::ALL {
            assert_eq!(Locale::parse(locale.code()), Some(locale));
        }
        assert_eq!(Locale::parse("pt"), None);
        assert_eq!(Locale::parse(""), None);
    }

    #[test]
    fn cookie_header_yields_the_chosen_language() {
        assert_eq!(
            Locale::from_cookie_header("a=1; iw-lang=de; b=2"),
            Some(Locale::De)
        );
        assert_eq!(Locale::from_cookie_header("iw-lang=zh"), Some(Locale::Zh));
        assert_eq!(Locale::from_cookie_header("iw-lang=xx"), None);
        assert_eq!(Locale::from_cookie_header("other-iw-lang=fr"), None);
        assert_eq!(Locale::from_cookie_header(""), None);
    }

    #[test]
    fn default_language_is_english() {
        assert_eq!(Locale::default(), Locale::En);
        assert_eq!(LocaleState::default().locale, Locale::En);
        assert!(!LocaleState::default().chosen);
    }

    #[test]
    fn msg_returns_the_text_of_each_language() {
        assert_eq!(GREETING.in_locale(Locale::En), "Hello {name}");
        assert_eq!(GREETING.in_locale(Locale::Es), "Hola {name}");
        assert_eq!(GREETING.in_locale(Locale::Zh), "你好 {name}");
        assert_eq!(GREETING.in_locale(Locale::De), "Hallo {name}");
        assert_eq!(GREETING.in_locale(Locale::Fr), "Bonjour {name}");
    }

    #[test]
    fn msg_get_uses_english_without_a_locale_context() {
        assert_eq!(GREETING.get(), "Hello {name}");
    }

    #[test]
    fn msg_get_and_fmt_follow_the_provided_locale() {
        let owner: Owner = Owner::new();
        owner.with(|| {
            provide_context(LocaleState {
                locale: Locale::Fr,
                chosen: true,
            });
            assert_eq!(GREETING.get(), "Bonjour {name}");
            assert_eq!(GREETING.fmt(&[("name", "Ana")]), "Bonjour Ana");
        });
    }

    #[test]
    fn fmt_replaces_every_occurrence_of_each_placeholder() {
        const TWICE: Msg = Msg {
            en: "{a}-{b}-{a}",
            es: "{a}-{b}-{a}",
            zh: "{a}-{b}-{a}",
            de: "{a}-{b}-{a}",
            fr: "{a}-{b}-{a}",
        };
        assert_eq!(TWICE.fmt(&[("a", "1"), ("b", "2")]), "1-2-1");
    }
}
