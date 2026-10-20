use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::busy::BusyButton;
use leptos::wasm_bindgen::JsCast;
use leptos::web_sys::{FormData, HtmlFormElement};

use crate::core::components::form::TextField;
use crate::core::i18n::Msg;

const DESCRIPTION: Msg = Msg {
    en: "Description",
    es: "Descripción",
    zh: "描述",
    de: "Beschreibung",
    fr: "Description",
};
const DESCRIPTION_PLACEHOLDER: Msg = Msg {
    en: "Rules for Rust projects",
    es: "Reglas para proyectos Rust",
    zh: "Rust 项目的规则",
    de: "Regeln für Rust-Projekte",
    fr: "Règles pour les projets Rust",
};
const FILE_LABEL: Msg = Msg {
    en: "AGENTS.md file",
    es: "Fichero AGENTS.md",
    zh: "AGENTS.md 文件",
    de: "AGENTS.md-Datei",
    fr: "Fichier AGENTS.md",
};
const UPLOADING: Msg = Msg {
    en: "Uploading…",
    es: "Subiendo…",
    zh: "上传中…",
    de: "Wird hochgeladen…",
    fr: "Envoi en cours…",
};
const UPLOAD: Msg = Msg {
    en: "Upload",
    es: "Subir",
    zh: "上传",
    de: "Hochladen",
    fr: "Envoyer",
};

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
                label=DESCRIPTION.get()
                value=description
                placeholder=DESCRIPTION_PLACEHOLDER.get()
                required=true
                name="description"
            />
            <label class="stack gap-1.5">
                <span class="text-sm font-medium text-fg">{FILE_LABEL.get()}</span>
                <input
                    type="file"
                    name="file"
                    accept=".md,text/markdown,text/plain"
                    required=true
                    class="input"
                />
            </label>
            <div class="row">
                <BusyButton r#type="submit" class="btn-primary" busy=pending>
                    {move || if pending.get() { UPLOADING.get() } else { UPLOAD.get() }}
                </BusyButton>
            </div>
        </form>
    }
}
