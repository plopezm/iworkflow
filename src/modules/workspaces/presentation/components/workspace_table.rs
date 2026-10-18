use crate::core::dock::link::TabLink;
use leptos::prelude::*;

use crate::core::browser::confirm;
use crate::core::i18n::Msg;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::WorkspaceDto;

const CONFIRM_REMOVE: Msg = Msg {
    en: "Remove the workspace \"{name}\"?",
    es: "¿Quitar el workspace \"{name}\"?",
    zh: "要移除工作区“{name}”吗？",
    de: "Workspace „{name}“ entfernen?",
    fr: "Retirer l'espace de travail « {name} » ?",
};
const NO_WORKSPACES: Msg = Msg {
    en: "No workspaces registered yet.",
    es: "Aún no hay workspaces registrados.",
    zh: "还没有已注册的工作区。",
    de: "Noch keine Workspaces registriert.",
    fr: "Aucun espace de travail enregistré pour le moment.",
};
const EDIT: Msg = Msg {
    en: "Edit",
    es: "Editar",
    zh: "编辑",
    de: "Bearbeiten",
    fr: "Modifier",
};
const OPEN_TERMINAL: Msg = Msg {
    en: "Open terminal",
    es: "Abrir terminal",
    zh: "打开终端",
    de: "Terminal öffnen",
    fr: "Ouvrir un terminal",
};
const REMOVE: Msg = Msg {
    en: "Remove",
    es: "Quitar",
    zh: "移除",
    de: "Entfernen",
    fr: "Retirer",
};

/// Ejecuta `on_delete` con `id` solo si el usuario confirma quitar el workspace `name`.
fn confirm_delete(name: &str, id: &str, on_delete: Callback<String>) {
    if confirm(&CONFIRM_REMOVE.fmt(&[("name", name)])) {
        on_delete.run(id.to_string());
    }
}

/// Listado de workspaces registrados: tabla en escritorio, tarjetas en móvil.
#[component]
pub fn WorkspaceTable(workspaces: Vec<WorkspaceDto>, on_delete: Callback<String>) -> impl IntoView {
    if workspaces.is_empty() {
        return view! { <p class="muted">{NO_WORKSPACES.get()}</p> }.into_any();
    }

    view! {
        <div class="stack sm:hidden">
            {workspaces
                .into_iter()
                .map(|w: WorkspaceDto| {
                    let id: String = w.id.clone();
                    let name: String = w.name.clone();
                    let edit_href: String = format!("/workspaces/{}/edit", w.id);
                    let terminal_href: String = format!("/workspaces/{}/terminal", w.id);
                    view! {
                        <article class="card stack gap-1.5 animate-slide-up">
                            <div class="row justify-between">
                                <span class="font-medium text-fg">
                                    <span class="mr-1.5 inline-block h-2 w-2 rounded-full bg-accent"></span>
                                    {w.name}
                                </span>
                                <div class="row gap-1.5">
                                <TabLink
                                    href=edit_href
                                    class="btn-secondary min-h-9 rounded-lg px-2.5 py-1 text-xs"
                                >
                                    {EDIT.get()}
                                </TabLink>
                                <TabLink
                                    href=terminal_href
                                    class="btn-secondary min-h-9 rounded-lg px-2.5 py-1 text-xs"
                                >
                                    {OPEN_TERMINAL.get()}
                                </TabLink>
                                <button
                                    class="btn-danger min-h-9 rounded-lg px-2.5 py-1 text-xs"
                                    on:click=move |_| confirm_delete(&name, &id, on_delete)
                                >
                                    {REMOVE.get()}
                                </button>
                                </div>
                            </div>
                            <code class="truncate font-mono text-xs text-muted">{w.path}</code>
                            <code class="font-mono text-xs text-muted">{w.base_branch}</code>
                        </article>
                    }
                })
                .collect_view()}
        </div>
    }
        .into_any()
}
