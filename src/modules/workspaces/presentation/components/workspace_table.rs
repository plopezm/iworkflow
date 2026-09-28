use leptos::prelude::*;

use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::WorkspaceDto;

/// Tabla de workspaces registrados con acción de borrado.
#[component]
pub fn WorkspaceTable(workspaces: Vec<WorkspaceDto>, on_delete: Callback<String>) -> impl IntoView {
    if workspaces.is_empty() {
        return view! { <p class="muted">"Aún no hay workspaces registrados."</p> }.into_any();
    }

    view! {
        <table>
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
                    .into_iter()
                    .map(|w: WorkspaceDto| {
                        let id: String = w.id.clone();
                        view! {
                            <tr>
                                <td>{w.name}</td>
                                <td>
                                    <code>{w.path}</code>
                                </td>
                                <td>
                                    <code>{w.base_branch}</code>
                                </td>
                                <td>
                                    <button
                                        class="danger"
                                        on:click=move |_| on_delete.run(id.clone())
                                    >
                                        "Quitar"
                                    </button>
                                </td>
                            </tr>
                        }
                    })
                    .collect_view()}
            </tbody>
        </table>
    }
    .into_any()
}
