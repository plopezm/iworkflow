use leptos::prelude::*;
use leptos_meta::Title;
use leptos_router::components::A;
use leptos_router::hooks::use_params_map;

use crate::core::components::layout::ErrorText;
use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::{
    ProjectDirectoryDto, TreeEntryDto,
};
use crate::modules::project_tree::presentation::components::tree_node::TreeNode;
use crate::modules::project_tree::presentation::state::project_tree_state::ProjectTreeState;

/// Árbol de directorios de un workspace. Se reconstruye por cada id (el router reutiliza el componente).
#[component]
pub fn ProjectTreePage() -> impl IntoView {
    let params = use_params_map();
    let workspace_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <ProjectTreeContent workspace_id=workspace_id.get() /> }} }
}

#[component]
fn ProjectTreeContent(workspace_id: String) -> impl IntoView {
    let state: ProjectTreeState = ProjectTreeState::new(workspace_id.clone());
    let edit_href: String = format!("/workspaces/{workspace_id}/edit");
    let terminal_href: String = format!("/workspaces/{workspace_id}/terminal");

    view! {
        <Title text="Proyecto · iworkflow" />
        <div class="h-full min-h-0 overflow-y-auto">
            <div class="mx-auto w-full max-w-4xl p-4 pb-10 md:p-6 md:pb-12">
                <div class="stack gap-4">
                    <div class="row flex-wrap items-center justify-between gap-2">
                        <h1 class="page-title">
                            <Transition fallback=|| "Proyecto">
                                {move || Suspend::new(async move {
                                    match state.root.await {
                                        Ok(directory) => directory.workspace_name,
                                        Err(_) => "Proyecto".to_string(),
                                    }
                                })}
                            </Transition>
                        </h1>
                        <div class="row gap-2">
                            <A href=edit_href attr:class="btn-secondary min-h-9 px-3 py-1.5 text-xs">
                                "Editar"
                            </A>
                            <A
                                href=terminal_href
                                attr:class="btn-secondary min-h-9 px-3 py-1.5 text-xs"
                            >
                                "Terminal"
                            </A>
                        </div>
                    </div>
                    <section class="card p-2">
                        <div class="max-h-[60vh] overflow-y-auto font-mono text-xs md:text-sm">
                            <Transition fallback=|| {
                                view! { <p class="muted p-2">"Cargando proyecto…"</p> }
                            }>
                                {move || Suspend::new(async move {
                                    match state.root.await {
                                        Err(e) => {
                                            view! { <ErrorText message=e.to_string() /> }.into_any()
                                        }
                                        Ok(ProjectDirectoryDto { entries, .. }) if entries.is_empty() => {
                                            view! { <p class="muted p-2">"Carpeta vacía."</p> }
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
                    </section>
                </div>
            </div>
        </div>
    }
}
