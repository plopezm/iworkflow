use crate::core::dock::context::{use_tab_nav, use_tab_params};
use leptos::prelude::*;

use crate::core::components::form::{ActionError, SelectField, SelectOption, TextField};
use crate::core::components::layout::ErrorText;
use crate::core::i18n::Msg;
use crate::modules::workspaces::presentation::state::workspace_edit_state::{
    WorkspaceEditState, split_agents_options,
};

const WORKSPACES_HREF: &str = "/workspaces";

const LOADING_WORKSPACE: Msg = Msg {
    en: "Loading workspace…",
    es: "Cargando workspace…",
    zh: "正在加载工作区…",
    de: "Workspace wird geladen…",
    fr: "Chargement de l'espace de travail…",
};
const EDIT_WORKSPACE: Msg = Msg {
    en: "Edit workspace",
    es: "Editar workspace",
    zh: "编辑工作区",
    de: "Workspace bearbeiten",
    fr: "Modifier l'espace de travail",
};
const NAME: Msg = Msg {
    en: "Name",
    es: "Nombre",
    zh: "名称",
    de: "Name",
    fr: "Nom",
};
const NAME_PLACEHOLDER: Msg = Msg {
    en: "my-project",
    es: "mi-proyecto",
    zh: "我的项目",
    de: "mein-projekt",
    fr: "mon-projet",
};
const PATH: Msg = Msg {
    en: "Path",
    es: "Ruta",
    zh: "路径",
    de: "Pfad",
    fr: "Chemin",
};
const BASE_BRANCH: Msg = Msg {
    en: "Base branch",
    es: "Rama base",
    zh: "基础分支",
    de: "Basis-Branch",
    fr: "Branche de base",
};
const REPLACE_AGENTS: Msg = Msg {
    en: "Replace AGENTS.md (optional)",
    es: "Sustituir AGENTS.md (opcional)",
    zh: "替换 AGENTS.md（可选）",
    de: "AGENTS.md ersetzen (optional)",
    fr: "Remplacer AGENTS.md (facultatif)",
};
const KEEP_CURRENT: Msg = Msg {
    en: "Keep the current one",
    es: "Mantener el actual",
    zh: "保留当前文件",
    de: "Aktuelle beibehalten",
    fr: "Conserver l'actuel",
};
const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};
const SAVING: Msg = Msg {
    en: "Saving…",
    es: "Guardando…",
    zh: "保存中…",
    de: "Wird gespeichert…",
    fr: "Enregistrement…",
};
const SAVE_CHANGES: Msg = Msg {
    en: "Save changes",
    es: "Guardar cambios",
    zh: "保存更改",
    de: "Änderungen speichern",
    fr: "Enregistrer les modifications",
};

/// Página de edición. Se reconstruye por cada id (el router reutiliza el componente).
#[component]
pub fn WorkspaceEditPage() -> impl IntoView {
    let params = use_tab_params();
    let workspace_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <WorkspaceEditContent workspace_id=workspace_id.get() /> }} }
}

#[component]
fn WorkspaceEditContent(workspace_id: String) -> impl IntoView {
    let state: WorkspaceEditState = WorkspaceEditState::new(workspace_id);
    let nav = use_tab_nav();
    let leave: Callback<()> = Callback::new(move |_| {
        nav.close_and_open(WORKSPACES_HREF);
    });

    // Precarga una sola vez, cuando llega el workspace.
    let prefilled: RwSignal<bool> = RwSignal::new(false);
    Effect::new(move |_| {
        if let Some(Ok(workspace)) = state.workspace.get()
            && !prefilled.get_untracked()
        {
            state.prefill(&workspace);
            prefilled.set(true);
        }
    });
    Effect::new(move |_| {
        if state.saved() {
            leave.run(());
        }
    });

    view! {
        <div class="h-full min-h-0 overflow-y-auto">
            <div class="mx-auto w-full max-w-3xl p-4 pb-10 md:p-6 md:pb-12">
                <Transition fallback=|| view! { <p class="muted">{LOADING_WORKSPACE.get()}</p> }>
                    {move || Suspend::new(async move {
                        match state.workspace.await {
                            Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                            Ok(workspace) => {
                                let (options, options_error): (
                                    Vec<SelectOption>,
                                    Option<String>,
                                ) = split_agents_options(state.agents_file_options.await);
                                view! {
                                    <form
                                        class="card stack gap-4 p-5 md:p-6"
                                        on:submit=move |ev: leptos::ev::SubmitEvent| {
                                            ev.prevent_default();
                                            state.submit();
                                        }
                                    >
                                        <h1 class="page-title">{EDIT_WORKSPACE.get()}</h1>
                                        <TextField
                                            label=NAME.get()
                                            value=state.form_name
                                            placeholder=NAME_PLACEHOLDER.get()
                                            required=true
                                        />
                                        <div class="stack gap-1.5">
                                            <span class="text-sm font-medium text-fg">{PATH.get()}</span>
                                            <code class="truncate font-mono text-xs text-muted">
                                                {workspace.path}
                                            </code>
                                        </div>
                                        <TextField
                                            label=BASE_BRANCH.get()
                                            value=state.form_base_branch
                                            placeholder="main"
                                        />
                                        {options_error
                                            .map(|message: String| {
                                                view! { <ErrorText message /> }
                                            })}
                                        {(!options.is_empty())
                                            .then(|| {
                                                view! {
                                                    <SelectField
                                                        label=REPLACE_AGENTS.get()
                                                        value=state.form_agents_file_id
                                                        options
                                                        placeholder=KEEP_CURRENT.get()
                                                    />
                                                }
                                            })}
                                        <ActionError result=state.update.value().into() />
                                        <div class="row justify-end gap-2">
                                            <button
                                                type="button"
                                                class="btn-secondary"
                                                on:click=move |_| leave.run(())
                                            >
                                                {CANCEL.get()}
                                            </button>
                                            <button
                                                type="submit"
                                                class="btn-primary"
                                                disabled=move || state.update.pending().get()
                                            >
                                                {move || {
                                                    if state.update.pending().get() {
                                                        SAVING.get()
                                                    } else {
                                                        SAVE_CHANGES.get()
                                                    }
                                                }}
                                            </button>
                                        </div>
                                    </form>
                                }
                                    .into_any()
                            }
                        }
                    })}
                </Transition>
            </div>
        </div>
    }
}
