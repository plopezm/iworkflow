use leptos::prelude::*;

use crate::core::browser::confirm;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::WorkspaceDto;

/// Ejecuta `on_delete` con `id` solo si el usuario confirma quitar el workspace `name`.
fn confirm_delete(name: &str, id: &str, on_delete: Callback<String>) {
    if confirm(&format!("¿Quitar el workspace \"{name}\"?")) {
        on_delete.run(id.to_string());
    }
}

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
                        let name: String = w.name.clone();
                        view! {
                            <tr>
                                <td class="font-medium">{w.name}</td>
                                <td>
                                    <code class="font-mono text-xs">{w.path}</code>
                                </td>
                                <td>
                                    <code class="font-mono text-xs">{w.base_branch}</code>
                                </td>
                                <td>
                                    <button class="btn-danger" on:click=move |_| confirm_delete(&name, &id, on_delete)>
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
                    let name: String = w.name.clone();
                    view! {
                        <article class="card stack gap-1.5 animate-slide-up">
                            <div class="row justify-between">
                                <span class="font-medium text-fg">
                                    <span class="mr-1.5 inline-block h-2 w-2 rounded-full bg-accent"></span>
                                    {w.name}
                                </span>
                                <button
                                    class="btn-danger min-h-9 rounded-lg px-2.5 py-1 text-xs"
                                    on:click=move |_| confirm_delete(&name, &id, on_delete)
                                >
                                    "Quitar"
                                </button>
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
