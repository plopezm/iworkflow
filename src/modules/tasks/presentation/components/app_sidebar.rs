use leptos::prelude::*;
use leptos_router::components::A;

use crate::core::components::shell::{SidebarItem, SidebarSection};
use crate::modules::tasks::presentation::state::app_sidebar_state::{
    AppSidebarState, SidebarTaskRow, SidebarWorkspaceRow, task_path,
};

const MENU_ITEM_CLASS: &str = "menu-item";

/// Listados de workspaces y tareas de la barra lateral izquierda.
#[component]
pub fn AppSidebar() -> impl IntoView {
    let state: AppSidebarState = AppSidebarState::new();
    let workspace_count: Signal<usize> =
        Signal::derive(move || state.workspace_rows.with(Vec::len));
    let task_count: Signal<usize> = Signal::derive(move || state.task_rows.with(Vec::len));

    view! {
        <SidebarSection title="Workspaces" count=workspace_count>
            <For
                each=move || state.workspace_rows.get()
                key=|row: &SidebarWorkspaceRow| format!("{}|{}|{}", row.id, row.name, row.subtitle)
                children=move |row: SidebarWorkspaceRow| {
                    view! {
                        <SidebarItem
                            href=format!("/#lane-{}", row.id)
                            title=row.name
                            subtitle=row.subtitle
                            selected=Signal::derive(|| false)
                            menu=move || {
                                view! {
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
                <p class="muted">"Aún no hay workspaces."</p>
            </Show>
        </SidebarSection>
        <SidebarSection title="Tareas" count=task_count>
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
                <p class="muted">"Aún no hay tareas."</p>
            </Show>
        </SidebarSection>
    }
}
