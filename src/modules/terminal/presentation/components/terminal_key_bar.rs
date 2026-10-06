use leptos::prelude::*;

/// Teclas especiales ofrecidas bajo la terminal: (etiqueta, nombre de tecla entendido por `terminal.js`).
pub const TERMINAL_KEYS: [(&str, &str); 6] = [
    ("Tab", "tab"),
    ("↑", "up"),
    ("←", "left"),
    ("→", "right"),
    ("↓", "down"),
    ("Ctrl+C", "ctrl_c"),
];

/// Hilera de botones que emite la tecla pulsada mediante `on_key` (nombre de tecla).
#[component]
pub fn TerminalKeyBar(#[prop(into)] on_key: Callback<&'static str>) -> impl IntoView {
    view! {
        <div class="row flex-wrap gap-1.5" role="toolbar" aria-label="Teclas especiales">
            {TERMINAL_KEYS
                .iter()
                .map(|&(label, key)| {
                    view! {
                        <button
                            type="button"
                            class="btn-secondary min-h-8 px-3 py-1 text-xs"
                            // Evita que el botón robe el foco a la terminal.
                            on:mousedown=|ev| ev.prevent_default()
                            on:click=move |_| on_key.run(key)
                        >
                            {label}
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
        for &(_, key) in TERMINAL_KEYS.iter() {
            assert!(
                TERMINAL_JS.contains(&format!("{key}:")),
                "terminal.js no define la tecla `{key}`"
            );
        }
    }

    #[test]
    fn labels_are_unique_and_non_empty() {
        let mut labels: Vec<&str> = TERMINAL_KEYS.iter().map(|&(label, _)| label).collect();
        assert!(labels.iter().all(|label| !label.is_empty()));
        labels.sort_unstable();
        labels.dedup();
        assert_eq!(labels.len(), TERMINAL_KEYS.len());
    }

    #[test]
    fn rendered_bar_has_one_button_per_key() {
        use super::TerminalKeyBar;
        use leptos::prelude::*;

        let owner: Owner = Owner::new();
        owner.set();
        let html: String =
            view! { <TerminalKeyBar on_key=Callback::new(|_key: &'static str| {}) /> }.to_html();
        assert_eq!(html.matches("<button").count(), TERMINAL_KEYS.len());
        assert!(html.contains("Ctrl+C"));
    }

    #[test]
    fn exposes_the_six_requested_keys_with_unique_names() {
        let names: Vec<&str> = TERMINAL_KEYS.iter().map(|&(_, key)| key).collect();
        assert_eq!(names, ["tab", "up", "left", "right", "down", "ctrl_c"]);
    }
}
