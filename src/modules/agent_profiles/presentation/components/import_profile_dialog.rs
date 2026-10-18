use leptos::prelude::*;

use crate::core::browser::read_event_file_text;
use crate::core::components::form::{TextArea, TextField};
use crate::core::components::modal::Modal;
use crate::core::i18n::Msg;
use crate::modules::agent_profiles::presentation::state::name_rules::slug_hint;

const READ_FILE_FAILED: Msg = Msg {
    en: "Could not read the file: {reason}",
    es: "No se pudo leer el fichero: {reason}",
    zh: "无法读取文件：{reason}",
    de: "Die Datei konnte nicht gelesen werden: {reason}",
    fr: "Impossible de lire le fichier : {reason}",
};
const TITLE: Msg = Msg {
    en: "Import profile",
    es: "Importar perfil",
    zh: "导入配置文件",
    de: "Profil importieren",
    fr: "Importer un profil",
};
const FILE_LABEL: Msg = Msg {
    en: ".profile.json file",
    es: "Fichero .profile.json",
    zh: ".profile.json 文件",
    de: "Datei .profile.json",
    fr: "Fichier .profile.json",
};
const PASTE_JSON: Msg = Msg {
    en: "Or paste the JSON",
    es: "O pega el JSON",
    zh: "或粘贴 JSON",
    de: "Oder JSON einfügen",
    fr: "Ou collez le JSON",
};
const RENAME_LABEL: Msg = Msg {
    en: "New name (optional)",
    es: "Nuevo nombre (opcional)",
    zh: "新名称（可选）",
    de: "Neuer Name (optional)",
    fr: "Nouveau nom (facultatif)",
};
const RENAME_PLACEHOLDER: Msg = Msg {
    en: "Leave empty to keep the original",
    es: "Dejar vacío para conservar el original",
    zh: "留空则保留原名称",
    de: "Leer lassen, um das Original zu behalten",
    fr: "Laissez vide pour conserver l'original",
};
const IMPORTING: Msg = Msg {
    en: "Importing…",
    es: "Importando…",
    zh: "导入中…",
    de: "Wird importiert…",
    fr: "Importation…",
};
const IMPORT: Msg = Msg {
    en: "Import",
    es: "Importar",
    zh: "导入",
    de: "Importieren",
    fr: "Importer",
};
const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};

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
            Err(reason) => file_error.set(Some(READ_FILE_FAILED.fmt(&[("reason", &reason)]))),
        });
    };

    view! {
        <Modal title=TITLE.get() open on_close>
            <form
                class="stack"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    on_submit.run(());
                }
            >
                <label class="stack gap-1.5">
                    <span class="text-sm font-medium text-fg">{FILE_LABEL.get()}</span>
                    <input
                        type="file"
                        accept=".json,application/json"
                        class="input"
                        on:change=on_file
                    />
                </label>
                {move || file_error.get().map(|text: String| view! { <p class="text-sm text-error">{text}</p> })}
                <TextArea label=PASTE_JSON.get() value=raw placeholder="{ \"schema\": … }" required=true />
                <TextField
                    label=RENAME_LABEL.get()
                    value=rename_to
                    placeholder=RENAME_PLACEHOLDER.get()
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
                        {move || if pending.get() { IMPORTING.get() } else { IMPORT.get() }}
                    </button>
                    <button type="button" class="btn-secondary" on:click=move |_| on_close.run(())>
                        {CANCEL.get()}
                    </button>
                </div>
            </form>
        </Modal>
    }
}
