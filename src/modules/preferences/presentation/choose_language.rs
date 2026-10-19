use leptos::prelude::*;
use leptos::task::spawn_local;

use crate::core::i18n::{Locale, choose_locale};
use crate::modules::preferences::infrastructure::adapters::inbound::server_fns::save_language;

/// Persists the picked language on the server, then applies it in this browser (cookie + reload).
/// If the save fails the choice still applies locally, so the click is never lost.
pub fn choose_language(locale: Locale) {
    spawn_local(async move {
        let saved: Result<(), ServerFnError> = save_language(locale.code().to_string()).await;
        if let Err(error) = saved {
            leptos::logging::warn!("no se pudo guardar el idioma: {error}");
        }
        choose_locale(locale);
    });
}
