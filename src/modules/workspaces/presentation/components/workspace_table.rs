use leptos::prelude::*;

use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::WorkspaceDto;

/// Listado de workspaces registrados: tabla en escritorio, tarjetas en móvil.
#[component]
pub fn WorkspaceTable(workspaces: Vec<WorkspaceDto>, on_delete: Callback<String>) -> impl IntoView {
    if workspaces.is_empty() {
        return view! { <p class="muted">"Aún no hay workspaces registrados."</p> }.into_any();
    }

    view! {
        <table class="data-table hidden sm:table">
            <thead>
                <tr>
                    <th>"Nombre"</th>
                    <th>"Ruta"</th>
                    <th>"Rama base"</th>
                    <th></th>
                </tr>
            </thead>
            <tbody>
                {workspaces
                    .iter()
                    .cloned()
                    .map(|w: WorkspaceDto| {
                        let id: String = w.id.clone();
                        view! {
                            <tr>
                                <td>{w.name}</td>
                                <td>
                                    <code class="font-mono text-xs">{w.path}</code>
                                </td>
                                <td>
                                    <code class="font-mono text-xs">{w.base_branch}</code>
                                </td>
                                <td>
                                    <button class="btn-danger" on:click=move |_| on_delete.run(id.clone())>
                                        "Quitar"
                                    </button>
                                </td>
                            </tr>
                        }
                    })
                    .collect_view()}
            </tbody>
        </table>
        <div class="stack sm:hidden">
            {workspaces
                .into_iter()
                .map(|w: WorkspaceDto| {
                    let id: String = w.id.clone();
                    view! {
                        <article class="card stack gap-1.5">
                            <span class="font-medium text-fg">{w.name}</span>
                            <code class="truncate font-mono text-xs text-muted">{w.path}</code>
                            <code class="font-mono text-xs text-muted">{w.base_branch}</code>
                            <button
                                class="btn-danger self-start"
                                on:click=move |_| on_delete.run(id.clone())
                            >
                                "Quitar"
                            </button>
                        </article>
                    }
                })
                .collect_view()}
        </div>
    }
        .into_any()
}
