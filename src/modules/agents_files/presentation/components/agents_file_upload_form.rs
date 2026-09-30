use leptos::ev::SubmitEvent;
use leptos::prelude::*;
use leptos::wasm_bindgen::JsCast;
use leptos::web_sys::{FormData, HtmlFormElement};

use crate::core::components::form::TextField;

/// Formulario de subida multipart (campos `description` y `file`). Entrega el `FormData`
/// construido a `on_submit`.
#[component]
pub fn AgentsFileUploadForm(
    description: RwSignal<String>,
    form_ref: NodeRef<leptos::html::Form>,
    #[prop(into)] pending: Signal<bool>,
    on_submit: Callback<FormData>,
) -> impl IntoView {
    let submit = move |ev: SubmitEvent| {
        ev.prevent_default();
        let form: Option<HtmlFormElement> = ev
            .target()
            .and_then(|t: leptos::web_sys::EventTarget| t.dyn_into::<HtmlFormElement>().ok());
        if let Some(data) = form.and_then(|f: HtmlFormElement| FormData::new_with_form(&f).ok()) {
            on_submit.run(data);
        }
    };

    view! {
        <form class="stack" node_ref=form_ref on:submit=submit>
            <TextField
                label="Descripción"
                value=description
                placeholder="Reglas para proyectos Rust"
                required=true
                name="description"
            />
            <label class="stack gap-1.5">
                <span class="text-sm font-medium text-fg">"Fichero AGENTS.md"</span>
                <input
                    type="file"
                    name="file"
                    accept=".md,text/markdown,text/plain"
                    required=true
                    class="input"
                />
            </label>
            <div class="row">
                <button type="submit" class="btn-primary" disabled=move || pending.get()>
                    {move || if pending.get() { "Subiendo…" } else { "Subir" }}
                </button>
            </div>
        </form>
    }
}
