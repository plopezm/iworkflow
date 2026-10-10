use leptos::prelude::*;
use leptos_router::components::A;
use leptos_router::hooks::use_location;

use crate::core::components::shell::{SidebarItem, SidebarSection};
use crate::modules::tasks::presentation::state::app_sidebar_state::{
    AppSidebarState, SidebarTaskRow, SidebarWorkspaceRow, task_path,
};

const MENU_ITEM_CLASS: &str = "menu-item";

/// Route of the project tree page of a workspace (pure).
fn workspace_path(workspace_id: &str) -> String {
    format!("/workspaces/{workspace_id}")
}

/// Whether `pathname` is the project tree page of `workspace_id` (pure).
fn is_workspace_selected(pathname: &str, workspace_id: &str) -> bool {
    pathname == workspace_path(workspace_id)
}

/// Listados de workspaces y tareas de la barra lateral izquierda.
#[component]
pub fn AppSidebar() -> impl IntoView {
    let state: AppSidebarState = AppSidebarState::new();
    let workspace_count: Signal<usize> =
        Signal::derive(move || state.workspace_rows.with(Vec::len));
    let task_count: Signal<usize> = Signal::derive(move || state.task_rows.with(Vec::len));
    let pathname: Memo<String> = use_location().pathname;

    view! {
        <SidebarSection title="Workspaces" count=workspace_count>
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
                                pathname.with(|p: &String| is_workspace_selected(p, &selected_id))
                            })
                            menu=move || {
                                view! {
                                    <A href=board_href.clone() attr:class=MENU_ITEM_CLASS>
                                        "Ver en el tablero"
                                    </A>
                                    <A href=terminal_href.clone() attr:class=MENU_ITEM_CLASS>
                                        "Abrir terminal"
                                    </A>
                                    <A href="/workspaces" attr:class=MENU_ITEM_CLASS>
                                        "Gestionar workspaces"
                                    </A>
                                }
                            }
                        />
                    }
                }
            />
            <Show when=move || workspace_count.get() == 0>
                <div class="stack gap-1.5">
                    <p class="muted">"Aún no hay workspaces."</p>
                    <A href="/workspaces" attr:class="btn-ghost min-h-9 px-3 py-1.5 text-xs">
                        "Registrar workspace"
                    </A>
                </div>
            </Show>
        </SidebarSection>
        <SidebarSection title="Requieren atención" count=task_count>
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
                                    <A href=menu_path.clone() attr:class=MENU_ITEM_CLASS>
                                        "Abrir tarea"
                                    </A>
                                }
                            }
                        />
                    }
                }
            />
            <Show when=move || task_count.get() == 0>
                <p class="muted">"Nada requiere atención."</p>
            </Show>
        </SidebarSection>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn workspace_selected_only_on_its_own_tree_route() {
        assert_eq!(workspace_path("w1"), "/workspaces/w1");
        assert!(is_workspace_selected("/workspaces/w1", "w1"));
        assert!(!is_workspace_selected("/workspaces/w1/edit", "w1"));
        assert!(!is_workspace_selected("/workspaces/w2", "w1"));
    }
}
