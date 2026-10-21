use leptos::prelude::*;

use crate::core::i18n::Msg;

const SPECIAL_KEYS: Msg = Msg {
    en: "Special keys",
    es: "Teclas especiales",
    zh: "特殊按键",
    de: "Sondertasten",
    fr: "Touches spéciales",
};

/// Teclas ofrecidas bajo la terminal: (nombre accesible, nombre de tecla entendido por
/// `terminal.js`, contenido SVG del icono en una rejilla de 24x24).
pub const TERMINAL_KEYS: [(&str, &str, &str); 9] = [
    (
        "Esc",
        "esc",
        r#"<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6M15 9l-6 6"/>"#,
    ),
    (
        "Tab",
        "tab",
        r#"<path d="M3 12h14"/><path d="m13 8 4 4-4 4"/><path d="M21 5v14"/>"#,
    ),
    (
        "Up",
        "up",
        r#"<path d="m5 12 7-7 7 7"/><path d="M12 19V5"/>"#,
    ),
    (
        "Left",
        "left",
        r#"<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>"#,
    ),
    (
        "Right",
        "right",
        r#"<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>"#,
    ),
    (
        "Down",
        "down",
        r#"<path d="M12 5v14"/><path d="m19 12-7 7-7-7"/>"#,
    ),
    (
        "Ctrl+C",
        "ctrl_c",
        r#"<text x="12" y="16" text-anchor="middle" font-size="11" font-weight="700" fill="currentColor" stroke="none">^C</text>"#,
    ),
    (
        "Copy",
        "copy",
        r#"<rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>"#,
    ),
    (
        "Paste",
        "paste",
        r#"<rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>"#,
    ),
];

/// Hilera de botones con icono que emite la tecla pulsada mediante `on_key` (nombre de tecla).
#[component]
pub fn TerminalKeyBar(#[prop(into)] on_key: Callback<&'static str>) -> impl IntoView {
    view! {
        <div class="grid shrink-0 grid-cols-9 gap-1.5" role="toolbar" aria-label=SPECIAL_KEYS.get()>
            {TERMINAL_KEYS
                .iter()
                .map(|&(label, key, icon)| {
                    view! {
                        <button
                            type="button"
                            class="btn-secondary min-h-10 px-0 py-1"
                            aria-label=label
                            title=label
                            // Evita que el botón robe el foco a la terminal.
                            on:mousedown=|ev| ev.prevent_default()
                            on:click=move |_| on_key.run(key)
                        >
                            <svg
                                class="h-5 w-5"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                aria-hidden="true"
                                inner_html=icon
                            ></svg>
                        </button>
                    }
                })
                .collect_view()}
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::TERMINAL_KEYS;

    #[test]
    fn every_key_name_is_handled_by_terminal_js() {
        const TERMINAL_JS: &str = include_str!("../../../../../public/terminal.js");
        for &(_, key, _) in TERMINAL_KEYS.iter() {
            assert!(
                TERMINAL_JS.contains(&format!("{key}:")),
                "terminal.js no define la tecla `{key}`"
            );
        }
    }

    #[test]
    fn labels_are_unique_and_non_empty() {
        let mut labels: Vec<&str> = TERMINAL_KEYS.iter().map(|&(label, _, _)| label).collect();
        assert!(labels.iter().all(|label| !label.is_empty()));
        labels.sort_unstable();
        labels.dedup();
        assert_eq!(labels.len(), TERMINAL_KEYS.len());
    }

    // Renderizar a HTML exige `ssr` y no funciona con `hydrate` (solo wasm).
    #[cfg(all(feature = "ssr", not(feature = "hydrate")))]
    #[test]
    fn rendered_bar_has_one_button_per_key() {
        use super::TerminalKeyBar;
        use leptos::prelude::*;

        let owner: Owner = Owner::new();
        owner.set();
        let html: String =
            view! { <TerminalKeyBar on_key=Callback::new(|_key: &'static str| {}) /> }.to_html();
        assert_eq!(html.matches("<button").count(), TERMINAL_KEYS.len());
        assert!(html.contains(r#"aria-label="Ctrl+C""#));
        assert!(html.contains(r#"aria-label="Copy""#));
        assert!(html.contains(r#"aria-label="Paste""#));
    }

    #[test]
    fn exposes_the_requested_keys_with_unique_names() {
        let names: Vec<&str> = TERMINAL_KEYS.iter().map(|&(_, key, _)| key).collect();
        assert_eq!(
            names,
            [
                "esc", "tab", "up", "left", "right", "down", "ctrl_c", "copy", "paste"
            ]
        );
    }
}
