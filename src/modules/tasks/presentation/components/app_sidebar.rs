use crate::core::dock::link::TabLink;
use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::core::components::shell::{SidebarItem, SidebarSection};
use crate::modules::tasks::presentation::state::app_sidebar_state::{
    AppSidebarState, SidebarTaskRow, SidebarWorkspaceRow, task_path,
};

const MENU_ITEM_CLASS: &str = "menu-item";
const WORKSPACES: Msg = Msg {
    en: "Workspaces",
    es: "Workspaces",
    zh: "工作区",
    de: "Workspaces",
    fr: "Espaces de travail",
};

const VIEW_ON_BOARD: Msg = Msg {
    en: "View on board",
    es: "Ver en el tablero",
    zh: "在看板中查看",
    de: "Auf dem Board anzeigen",
    fr: "Voir sur le tableau",
};

const OPEN_TERMINAL: Msg = Msg {
    en: "Open terminal",
    es: "Abrir terminal",
    zh: "打开终端",
    de: "Terminal öffnen",
    fr: "Ouvrir le terminal",
};

const MANAGE_WORKSPACES: Msg = Msg {
    en: "Manage workspaces",
    es: "Gestionar workspaces",
    zh: "管理工作区",
    de: "Workspaces verwalten",
    fr: "Gérer les espaces de travail",
};

const NO_WORKSPACES: Msg = Msg {
    en: "No workspaces yet.",
    es: "Aún no hay workspaces.",
    zh: "还没有工作区。",
    de: "Noch keine Workspaces vorhanden.",
    fr: "Aucun espace de travail pour l’instant.",
};

const REGISTER_WORKSPACE: Msg = Msg {
    en: "Register workspace",
    es: "Registrar workspace",
    zh: "注册工作区",
    de: "Workspace registrieren",
    fr: "Enregistrer un espace de travail",
};

const NEEDS_ATTENTION: Msg = Msg {
    en: "Needs attention",
    es: "Requieren atención",
    zh: "需要关注",
    de: "Erfordern Aufmerksamkeit",
    fr: "Nécessitent votre attention",
};

const OPEN_TASK: Msg = Msg {
    en: "Open task",
    es: "Abrir tarea",
    zh: "打开任务",
    de: "Aufgabe öffnen",
    fr: "Ouvrir la tâche",
};

const NOTHING_NEEDS_ATTENTION: Msg = Msg {
    en: "Nothing needs attention.",
    es: "Nada requiere atención.",
    zh: "没有需要关注的内容。",
    de: "Nichts erfordert Aufmerksamkeit.",
    fr: "Rien ne nécessite votre attention.",
};

/// Route of the project tree page of a workspace (pure).
fn workspace_path(workspace_id: &str) -> String {
    format!("/workspaces/{workspace_id}")
}

/// Listados de workspaces y tareas de la barra lateral izquierda.
#[component]
pub fn AppSidebar() -> impl IntoView {
    let state: AppSidebarState = AppSidebarState::new();
    let workspace_count: Signal<usize> =
        Signal::derive(move || state.workspace_rows.with(Vec::len));
    let task_count: Signal<usize> = Signal::derive(move || state.task_rows.with(Vec::len));

    view! {
        <SidebarSection title=WORKSPACES.get() count=workspace_count>
            <For
                each=move || state.workspace_rows.get()
                key=|row: &SidebarWorkspaceRow| format!("{}|{}|{}", row.id, row.name, row.subtitle)
                children=move |row: SidebarWorkspaceRow| {
                    let terminal_href: String = format!("/workspaces/{}/terminal", row.id);
                    let board_href: String = format!("/#lane-{}", row.id);
                    let selected_id: String = row.id.clone();
                    view! {
                        <SidebarItem
                            href=workspace_path(&row.id)
                            title=row.name
                            subtitle=row.subtitle
                            selected=Signal::derive(move || {
                                state.is_path_visible(&workspace_path(&selected_id))
                            })
                            menu=move || {
                                view! {
                                    <TabLink href=board_href.clone() class=MENU_ITEM_CLASS>
                                        {VIEW_ON_BOARD.get()}
                                    </TabLink>
                                    <TabLink href=terminal_href.clone() class=MENU_ITEM_CLASS>
                                        {OPEN_TERMINAL.get()}
                                    </TabLink>
                                    <TabLink href="/workspaces" class=MENU_ITEM_CLASS>
                                        {MANAGE_WORKSPACES.get()}
                                    </TabLink>
                                }
                            }
                        />
                    }
                }
            />
            <Show when=move || workspace_count.get() == 0>
                <div class="stack gap-1.5">
                    <p class="muted">{NO_WORKSPACES.get()}</p>
                    <TabLink href="/workspaces" class="btn-ghost min-h-9 px-3 py-1.5 text-xs">
                        {REGISTER_WORKSPACE.get()}
                    </TabLink>
                </div>
            </Show>
        </SidebarSection>
        <SidebarSection title=NEEDS_ATTENTION.get() count=task_count>
            <For
                each=move || state.task_rows.get()
                key=|row: &SidebarTaskRow| {
                    format!("{}|{}|{}|{}", row.id, row.title, row.subtitle, row.needs_attention)
                }
                children=move |row: SidebarTaskRow| {
                    let id: String = row.id.clone();
                    let path: String = task_path(&row.id);
                    let menu_path: String = path.clone();
                    view! {
                        <SidebarItem
                            href=path
                            title=row.title
                            subtitle=row.subtitle
                            highlight=row.needs_attention
                            selected=Signal::derive(move || state.is_task_selected(&id))
                            menu=move || {
                                view! {
                                    <TabLink href=menu_path.clone() class=MENU_ITEM_CLASS>
                                        {OPEN_TASK.get()}
                                    </TabLink>
                                }
                            }
                        />
                    }
                }
            />
            <Show when=move || task_count.get() == 0>
                <p class="muted">{NOTHING_NEEDS_ATTENTION.get()}</p>
            </Show>
        </SidebarSection>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn workspace_path_points_to_its_tree_route() {
        assert_eq!(workspace_path("w1"), "/workspaces/w1");
    }
}
