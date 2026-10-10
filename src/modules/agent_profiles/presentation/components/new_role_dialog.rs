use leptos::prelude::*;

use crate::core::components::form::{TextArea, TextField};
use crate::core::components::modal::Modal;
use crate::modules::agent_profiles::presentation::state::name_rules::slug_hint;

/// Diálogo para crear un rol de agente (fichero `<rol>.md`).
#[component]
pub fn NewRoleDialog(
    #[prop(into)] open: Signal<bool>,
    name: RwSignal<String>,
    description: RwSignal<String>,
    prompt: RwSignal<String>,
    #[prop(into)] pending: Signal<bool>,
    on_submit: Callback<()>,
    on_close: Callback<()>,
) -> impl IntoView {
    let hint: Memo<Option<&'static str>> = Memo::new(move |_| slug_hint(name.get().trim()));

    view! {
        <Modal title="Nuevo rol de agente" open on_close>
            <form
                class="stack"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    if hint.get_untracked().is_none() {
                        on_submit.run(());
                    }
                }
            >
                <TextField label="Nombre (slug)" value=name placeholder="sdd-review" required=true />
                {move || {
                    hint.get()
                        .filter(|_| !name.get().is_empty())
                        .map(|text: &'static str| view! { <p class="text-sm text-error">{text}</p> })
                }}
                <TextField
                    label="Descripción"
                    value=description
                    placeholder="Qué hace este agente"
                    required=true
                />
                <TextArea label="Prompt" value=prompt placeholder="Instrucciones del agente" required=true />
                <div class="row gap-2">
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || pending.get() || hint.get().is_some()
                    >
                        {move || if pending.get() { "Creando…" } else { "Crear rol" }}
                    </button>
                    <button type="button" class="btn-secondary" on:click=move |_| on_close.run(())>
                        "Cancelar"
                    </button>
                </div>
            </form>
        </Modal>
    }
}
