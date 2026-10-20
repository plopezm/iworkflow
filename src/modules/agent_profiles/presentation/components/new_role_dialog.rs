use leptos::prelude::*;

use crate::core::components::form::{TextArea, TextField};
use crate::core::components::modal::Modal;
use crate::core::i18n::Msg;
use crate::modules::agent_profiles::presentation::state::name_rules::slug_hint;

const TITLE: Msg = Msg {
    en: "New agent role",
    es: "Nuevo rol de agente",
    zh: "新建智能体角色",
    de: "Neue Agent-Rolle",
    fr: "Nouveau rôle d'agent",
};

const NAME_LABEL: Msg = Msg {
    en: "Name (slug)",
    es: "Nombre (slug)",
    zh: "名称（slug）",
    de: "Name (Slug)",
    fr: "Nom (slug)",
};

const DESCRIPTION_LABEL: Msg = Msg {
    en: "Description",
    es: "Descripción",
    zh: "描述",
    de: "Beschreibung",
    fr: "Description",
};

const DESCRIPTION_PLACEHOLDER: Msg = Msg {
    en: "What this agent does",
    es: "Qué hace este agente",
    zh: "此智能体的作用",
    de: "Was dieser Agent tut",
    fr: "Ce que fait cet agent",
};

const PROMPT_PLACEHOLDER: Msg = Msg {
    en: "Agent instructions",
    es: "Instrucciones del agente",
    zh: "智能体指令",
    de: "Anweisungen für den Agent",
    fr: "Instructions de l'agent",
};

const CREATING: Msg = Msg {
    en: "Creating…",
    es: "Creando…",
    zh: "创建中…",
    de: "Wird erstellt…",
    fr: "Création…",
};

const CREATE: Msg = Msg {
    en: "Create role",
    es: "Crear rol",
    zh: "创建角色",
    de: "Rolle erstellen",
    fr: "Créer le rôle",
};

const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};

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
        <Modal title=TITLE.get() open on_close>
            <form
                class="stack"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    if hint.get_untracked().is_none() {
                        on_submit.run(());
                    }
                }
            >
                <TextField label=NAME_LABEL.get() value=name placeholder="sdd-review" required=true />
                {move || {
                    hint.get()
                        .filter(|_| !name.get().is_empty())
                        .map(|text: &'static str| view! { <p class="text-sm text-error">{text}</p> })
                }}
                <TextField
                    label=DESCRIPTION_LABEL.get()
                    value=description
                    placeholder=DESCRIPTION_PLACEHOLDER.get()
                    required=true
                />
                <TextArea label="Prompt" value=prompt placeholder=PROMPT_PLACEHOLDER.get() required=true />
                <div class="row gap-2">
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || pending.get() || hint.get().is_some()
                        class:is-busy=move || pending.get()
                    >
                        {move || if pending.get() { CREATING.get() } else { CREATE.get() }}
                    </button>
                    <button type="button" class="btn-secondary" on:click=move |_| on_close.run(())>
                        {CANCEL.get()}
                    </button>
                </div>
            </form>
        </Modal>
    }
}
