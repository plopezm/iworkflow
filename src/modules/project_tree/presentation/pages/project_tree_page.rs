use crate::core::components::shell::DropdownMenu;
use crate::core::dock::context::use_tab_params;
use crate::core::dock::link::TabLink;
use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::{
    ProjectDirectoryDto, TreeEntryDto,
};
use crate::modules::project_tree::presentation::components::tree_node::TreeNode;
use crate::modules::project_tree::presentation::state::project_file_state::{
    edit_href, terminal_href,
};
use crate::modules::project_tree::presentation::state::project_tree_state::ProjectTreeState;

/// Árbol de directorios de un workspace. Se reconstruye por cada id (el router reutiliza el componente).
#[component]
pub fn ProjectTreePage() -> impl IntoView {
    let params = use_tab_params();
    let workspace_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <ProjectTreeContent workspace_id=workspace_id.get() /> }} }
}

#[component]
fn ProjectTreeContent(workspace_id: String) -> impl IntoView {
    let state: ProjectTreeState = ProjectTreeState::new(workspace_id.clone());
    let edit_href: Option<String> = edit_href(&workspace_id);
    let terminal_href: String = terminal_href(&workspace_id);

    view! {
        <div class="flex h-full min-h-0 flex-col bg-surface">
            <div class="tree-panel-header">
                <h1 class="min-w-0 flex-1 truncate text-sm font-semibold">
                    <Transition fallback=|| "Proyecto">
                        {move || Suspend::new(async move {
                            match state.root.await {
                                Ok(directory) => directory.workspace_name,
                                Err(_) => "Proyecto".to_string(),
                            }
                        })}
                    </Transition>
                </h1>
                <button
                    class="icon-btn"
                    title="Recargar"
                    aria-label="Recargar"
                    on:click=move |_| state.refresh()
                >
                    "↻"
                </button>
                <DropdownMenu
                    label="⋯"
                    aria_label="Acciones del proyecto"
                    trigger_class="icon-btn"
                    menu_class="min-w-44"
                >
                    <TabLink href=terminal_href.clone() class="menu-item">
                        "Abrir terminal"
                    </TabLink>
                    {edit_href
                        .clone()
                        .map(|href: String| {
                            view! {
                                <TabLink href=href class="menu-item">
                                    "Editar workspace"
                                </TabLink>
                            }
                        })}
                </DropdownMenu>
            </div>
            <div class="min-h-0 flex-1 overflow-y-auto py-1">
                <Transition fallback=|| {
                    view! { <p class="px-3 py-2 text-xs text-muted">"Cargando proyecto…"</p> }
                }>
                    {move || Suspend::new(async move {
                        match state.root.await {
                            Err(e) => {
                                view! {
                                    <div class="px-3 py-2">
                                        <ErrorText message=e.to_string() />
                                    </div>
                                }
                                    .into_any()
                            }
                            Ok(ProjectDirectoryDto { entries, .. }) if entries.is_empty() => {
                                view! { <p class="px-3 py-2 text-xs text-muted">"Carpeta vacía."</p> }
                                    .into_any()
                            }
                            Ok(ProjectDirectoryDto { entries, .. }) => {
                                view! {
                                    <ul>
                                        <For
                                            each=move || entries.clone()
                                            key=|e: &TreeEntryDto| e.path.clone()
                                            children=move |e: TreeEntryDto| {
                                                view! { <TreeNode entry=e depth=0 state /> }
                                            }
                                        />
                                    </ul>
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
