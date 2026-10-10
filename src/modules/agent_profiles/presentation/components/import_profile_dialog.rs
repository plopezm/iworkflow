use leptos::prelude::*;

use crate::core::browser::read_event_file_text;
use crate::core::components::form::{TextArea, TextField};
use crate::core::components::modal::Modal;
use crate::modules::agent_profiles::presentation::state::name_rules::slug_hint;

/// Diálogo de importación: JSON pegado o leído de un fichero, y nombre opcional.
#[component]
pub fn ImportProfileDialog(
    #[prop(into)] open: Signal<bool>,
    raw: RwSignal<String>,
    rename_to: RwSignal<String>,
    #[prop(into)] pending: Signal<bool>,
    on_submit: Callback<()>,
    on_close: Callback<()>,
) -> impl IntoView {
    let file_error: RwSignal<Option<String>> = RwSignal::new(None);
    let rename_hint: Memo<Option<&'static str>> = Memo::new(move |_| {
        let name: String = rename_to.get();
        let trimmed: &str = name.trim();
        if trimmed.is_empty() {
            None
        } else {
            slug_hint(trimmed)
        }
    });
    let on_file = move |ev: leptos::ev::Event| {
        file_error.set(None);
        read_event_file_text(ev, move |result: Result<String, String>| match result {
            Ok(text) => raw.set(text),
            Err(reason) => file_error.set(Some(format!("No se pudo leer el fichero: {reason}"))),
        });
    };

    view! {
        <Modal title="Importar perfil" open on_close>
            <form
                class="stack"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    on_submit.run(());
                }
            >
                <label class="stack gap-1.5">
                    <span class="text-sm font-medium text-fg">"Fichero .profile.json"</span>
                    <input
                        type="file"
                        accept=".json,application/json"
                        class="input"
                        on:change=on_file
                    />
                </label>
                {move || file_error.get().map(|text: String| view! { <p class="text-sm text-error">{text}</p> })}
                <TextArea label="O pega el JSON" value=raw placeholder="{ \"schema\": … }" required=true />
                <TextField
                    label="Nuevo nombre (opcional)"
                    value=rename_to
                    placeholder="Dejar vacío para conservar el original"
                />
                {move || rename_hint.get().map(|text: &'static str| view! { <p class="text-sm text-error">{text}</p> })}
                <div class="row gap-2">
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || {
                            pending.get() || raw.get().trim().is_empty() || rename_hint.get().is_some()
                        }
                    >
                        {move || if pending.get() { "Importando…" } else { "Importar" }}
                    </button>
                    <button type="button" class="btn-secondary" on:click=move |_| on_close.run(())>
                        "Cancelar"
                    </button>
                </div>
            </form>
        </Modal>
    }
}
