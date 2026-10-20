use leptos::prelude::*;

use crate::core::i18n::Msg;

const LOADING: Msg = Msg {
    en: "Loading",
    es: "Cargando",
    zh: "加载中",
    de: "Wird geladen",
    fr: "Chargement",
};

/// Small rotating ring that tells the user something is in progress.
#[component]
pub fn Spinner() -> impl IntoView {
    view! { <span class="spinner" role="status" aria-label=LOADING.get()></span> }
}

/// Placeholder for content that is still loading: spinner plus a short message.
#[component]
pub fn Loading(#[prop(into)] text: String) -> impl IntoView {
    view! {
        <p class="muted row gap-2" aria-live="polite">
            <span class="spinner" aria-hidden="true"></span>
            {text}
        </p>
    }
}

/// `true` once the browser has hydrated the page. Until then (server render, WASM still
/// downloading) event handlers are not attached yet, so controls must look busy instead of dead.
pub fn use_hydrated() -> Signal<bool> {
    let hydrated: RwSignal<bool> = RwSignal::new(false);
    // Effects only run in the browser, after hydration.
    Effect::new(move |_| hydrated.set(true));
    hydrated.into()
}

/// Class and `aria-busy` of a control that is waiting for input/output. `busy` wins over the
/// base class so a busy control is always rendered with the shared spinner and inert styling.
pub fn busy_class(base: &str, busy: bool) -> String {
    if busy {
        format!("{base} is-busy")
    } else {
        base.to_string()
    }
}

/// Button that shows a spinner and ignores clicks while `busy` is true.
///
/// Reuse it for any control whose action waits for the server or the disk, so every wait looks
/// the same. `disabled` blocks the button without the busy look.
#[component]
pub fn BusyButton(
    #[prop(into)] busy: Signal<bool>,
    #[prop(into, default = Signal::stored(false))] disabled: Signal<bool>,
    #[prop(into, default = "btn-secondary".to_string())] class: String,
    #[prop(into, default = "button".to_string())] r#type: String,
    #[prop(optional, into)] lang: Option<String>,
    #[prop(optional, into)] on_click: Option<Callback<()>>,
    children: Children,
) -> impl IntoView {
    view! {
        <button
            type=r#type
            lang=lang
            class=move || busy_class(&class, busy.get())
            aria-busy=move || busy.get().to_string()
            disabled=move || busy.get() || disabled.get()
            on:click=move |_| {
                if let Some(handler) = on_click {
                    handler.run(());
                }
            }
        >
            {children()}
        </button>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn busy_class_appends_the_busy_marker_only_while_busy() {
        assert_eq!(busy_class("btn-primary", false), "btn-primary");
        assert_eq!(busy_class("btn-primary", true), "btn-primary is-busy");
    }
}
