use std::collections::HashMap;

use leptos::prelude::*;
use leptos_router::components::A;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::presentation::components::task_status_badge::TaskStatusBadge;

/// Fecha RFC 3339 recortada a `AAAA-MM-DD HH:MM` para mostrarla.
pub fn short_timestamp(rfc3339: &str) -> String {
    rfc3339.get(..16).unwrap_or(rfc3339).replace('T', " ")
}

#[component]
pub fn TaskTable(tasks: Vec<TaskDto>, workspaces: Vec<WorkspaceOptionDto>) -> impl IntoView {
    if tasks.is_empty() {
        return view! { <p class="muted">"No hay tareas todavía."</p> }.into_any();
    }
    let names: HashMap<String, String> = workspaces
        .into_iter()
        .map(|w: WorkspaceOptionDto| (w.id, w.name))
        .collect();

    view! {
        <table>
            <thead>
                <tr>
                    <th>"Tarea"</th>
                    <th>"Workspace"</th>
                    <th>"Estado"</th>
                    <th>"Rama"</th>
                    <th>"Actualizada (UTC)"</th>
                </tr>
            </thead>
            <tbody>
                {tasks
                    .into_iter()
                    .map(|t: TaskDto| {
                        let workspace: String = names
                            .get(&t.workspace_id)
                            .cloned()
                            .unwrap_or_else(|| t.workspace_id.clone());
                        view! {
                            <tr>
                                <td>
                                    <A href=format!("/tasks/{}", t.id)>{t.title}</A>
                                </td>
                                <td>{workspace}</td>
                                <td>
                                    <TaskStatusBadge status=t.status />
                                </td>
                                <td>
                                    <code>{t.branch}</code>
                                </td>
                                <td class="muted">{short_timestamp(&t.updated_at)}</td>
                            </tr>
                        }
                    })
                    .collect_view()}
            </tbody>
        </table>
    }
    .into_any()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn short_timestamp_keeps_minutes() {
        assert_eq!(
            short_timestamp("2026-09-28T16:39:36.638+00:00"),
            "2026-09-28 16:39"
        );
        assert_eq!(short_timestamp("corto"), "corto");
    }
}
