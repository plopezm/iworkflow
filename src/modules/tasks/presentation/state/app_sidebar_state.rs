use std::collections::HashMap;

use leptos::prelude::*;
use leptos_router::NavigateOptions;
use leptos_router::hooks::{use_location, use_navigate};

use crate::core::components::browser_notification::{
    NotificationPermission, current_permission, is_page_inactive, request_permission,
    show_notification,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    list_tasks, list_workspace_options,
};
use crate::modules::tasks::presentation::state::task_events::subscribe_task_events;
use crate::modules::tasks::presentation::state::task_notifications::{
    TaskNotification, TaskSnapshot, build_snapshots, detect_task_notifications,
};

/// Fila de un workspace en la barra lateral.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SidebarWorkspaceRow {
    pub id: String,
    pub name: String,
    pub subtitle: String,
}

/// Fila de una tarea en la barra lateral.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SidebarTaskRow {
    pub id: String,
    pub title: String,
    pub subtitle: String,
    pub needs_attention: bool,
}

/// Ruta de detalle de una tarea.
pub fn task_path(task_id: &str) -> String {
    format!("/tasks/{task_id}")
}

/// Filas de workspaces con su rama base y el número de tareas que contienen.
pub fn build_workspace_rows(
    workspaces: &[WorkspaceOptionDto],
    tasks: &[TaskDto],
) -> Vec<SidebarWorkspaceRow> {
    workspaces
        .iter()
        .map(|w: &WorkspaceOptionDto| {
            let count: usize = tasks
                .iter()
                .filter(|t: &&TaskDto| t.workspace_id == w.id)
                .count();
            SidebarWorkspaceRow {
                id: w.id.clone(),
                name: w.name.clone(),
                subtitle: format!("{} · {count} tareas", w.base_branch),
            }
        })
        .collect()
}

/// Filas de las tareas que requieren atención, las actualizadas más recientemente primero
/// (RFC 3339 ordena como texto).
pub fn build_task_rows(
    tasks: &[TaskDto],
    workspaces: &[WorkspaceOptionDto],
) -> Vec<SidebarTaskRow> {
    let names: HashMap<&str, &str> = workspaces
        .iter()
        .map(|w: &WorkspaceOptionDto| (w.id.as_str(), w.name.as_str()))
        .collect();
    let mut sorted: Vec<&TaskDto> = tasks
        .iter()
        .filter(|t: &&TaskDto| t.needs_attention)
        .collect();
    sorted.sort_by(|a: &&TaskDto, b: &&TaskDto| b.updated_at.cmp(&a.updated_at));
    sorted
        .into_iter()
        .map(|t: &TaskDto| SidebarTaskRow {
            id: t.id.clone(),
            title: t.title.clone(),
            subtitle: match names.get(t.workspace_id.as_str()) {
                Some(name) => format!("{} · {name}", t.status.label()),
                None => t.status.label().to_string(),
            },
            needs_attention: t.needs_attention,
        })
        .collect()
}

/// Estado de la barra lateral: listados de workspaces y tareas (ya calculados, fuera de
/// `Suspend`) que se refrescan con SSE y al navegar.
#[derive(Clone, Copy)]
pub struct AppSidebarState {
    pub workspace_rows: RwSignal<Vec<SidebarWorkspaceRow>>,
    pub task_rows: RwSignal<Vec<SidebarTaskRow>>,
    pub pathname: Memo<String>,
    pub notification_permission: RwSignal<NotificationPermission>,
}

impl AppSidebarState {
    pub fn new() -> Self {
        let tick: RwSignal<u32> = RwSignal::new(0);
        let pathname: Memo<String> = use_location().pathname;
        // Needs the router context, so it is obtained here and not in the click handler.
        // The returned type is opaque (`impl Fn`), so it cannot be annotated.
        let navigate = use_navigate();
        let tasks: Resource<Result<Vec<TaskDto>, ServerFnError>> =
            Resource::new(move || (tick.get(), pathname.get()), |_| list_tasks(None));
        let workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>> = Resource::new(
            move || (tick.get(), pathname.get()),
            |_| list_workspace_options(),
        );
        let workspace_rows: RwSignal<Vec<SidebarWorkspaceRow>> = RwSignal::new(Vec::new());
        let task_rows: RwSignal<Vec<SidebarTaskRow>> = RwSignal::new(Vec::new());
        let notification_permission: RwSignal<NotificationPermission> =
            RwSignal::new(NotificationPermission::Unsupported);
        // Browser-only value: read after hydration to keep SSR and client markup equal.
        Effect::new(move |_| notification_permission.set(current_permission()));

        // Publica desde un `Effect` (solo cliente, tras hidratar) y conserva el valor
        // anterior mientras se recarga.
        Effect::new(move |last: Option<Option<HashMap<String, TaskSnapshot>>>| {
            let previous: Option<HashMap<String, TaskSnapshot>> = last.flatten();
            let loaded_workspaces: Option<Result<Vec<WorkspaceOptionDto>, ServerFnError>> =
                workspaces.get();
            let loaded_tasks: Option<Result<Vec<TaskDto>, ServerFnError>> = tasks.get();
            if let (Some(Ok(workspaces)), Some(Ok(tasks))) = (loaded_workspaces, loaded_tasks) {
                let next_workspaces: Vec<SidebarWorkspaceRow> =
                    build_workspace_rows(&workspaces, &tasks);
                let next_tasks: Vec<SidebarTaskRow> = build_task_rows(&tasks, &workspaces);
                if workspace_rows
                    .with_untracked(|c: &Vec<SidebarWorkspaceRow>| *c != next_workspaces)
                {
                    workspace_rows.set(next_workspaces);
                }
                if task_rows.with_untracked(|c: &Vec<SidebarTaskRow>| *c != next_tasks) {
                    task_rows.set(next_tasks);
                }
                // The first snapshot (page load) only seeds the baseline.
                if let Some(previous) = &previous {
                    notify_transitions(previous, &tasks, &navigate);
                }
                // The baseline always advances: a transition seen while the page is active
                // or without permission is intentionally not re-notified later (the sidebar
                // already shows the attention).
                return Some(build_snapshots(&tasks));
            }
            previous
        });
        subscribe_task_events(tick);

        Self {
            workspace_rows,
            task_rows,
            pathname,
            notification_permission,
        }
    }

    /// Asks for notification permission; must run from a user gesture (button click).
    pub fn enable_notifications(&self) {
        let permission: RwSignal<NotificationPermission> = self.notification_permission;
        request_permission(move |result: NotificationPermission| permission.set(result));
    }

    /// Whether the "enable notifications" prompt should be offered.
    pub fn can_request_notifications(&self) -> bool {
        self.notification_permission.get() == NotificationPermission::Default
    }

    /// Indica si `task_id` es la tarea abierta en la ruta actual.
    pub fn is_task_selected(&self, task_id: &str) -> bool {
        self.pathname.with(|p: &String| *p == task_path(task_id))
    }
}

/// Raises OS notifications for task transitions, only while the page is inactive.
fn notify_transitions<N>(previous: &HashMap<String, TaskSnapshot>, tasks: &[TaskDto], navigate: &N)
where
    N: Fn(&str, NavigateOptions) + Clone + 'static,
{
    if !is_page_inactive() {
        return;
    }
    for notification in detect_task_notifications(previous, tasks) {
        let TaskNotification {
            task_id,
            title,
            body,
        } = notification;
        let path: String = task_path(&task_id);
        let navigate: N = navigate.clone();
        let target: String = path.clone();
        show_notification(&title, &body, &path, move || {
            navigate(&target, NavigateOptions::default());
        });
    }
}

impl Default for AppSidebarState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
        TaskStageDto, TaskStatusDto,
    };

    fn workspace(id: &str, name: &str) -> WorkspaceOptionDto {
        WorkspaceOptionDto {
            id: id.to_string(),
            name: name.to_string(),
            base_branch: "main".to_string(),
        }
    }

    fn task(id: &str, workspace_id: &str, updated_at: &str) -> TaskDto {
        attention_task(id, workspace_id, updated_at, true)
    }

    fn attention_task(
        id: &str,
        workspace_id: &str,
        updated_at: &str,
        needs_attention: bool,
    ) -> TaskDto {
        TaskDto {
            id: id.to_string(),
            workspace_id: workspace_id.to_string(),
            title: format!("tarea {id}"),
            prompt: "prompt".to_string(),
            branch: "pi/x".to_string(),
            base_branch: "main".to_string(),
            worktree_path: "/wt".to_string(),
            status: TaskStatusDto::Idle,
            priority: Default::default(),
            error: None,
            actions: Default::default(),
            stage: TaskStageDto::Working,
            needs_attention,
            pending_dialogs: 0,
            created_at: updated_at.to_string(),
            updated_at: updated_at.to_string(),
        }
    }

    #[test]
    fn workspace_rows_count_their_tasks() {
        let workspaces: Vec<WorkspaceOptionDto> =
            vec![workspace("w1", "uno"), workspace("w2", "dos")];
        let tasks: Vec<TaskDto> = vec![
            task("a", "w1", "2026-01-01T00:00:00Z"),
            task("b", "w1", "2026-01-02T00:00:00Z"),
        ];
        let rows: Vec<SidebarWorkspaceRow> = build_workspace_rows(&workspaces, &tasks);
        assert_eq!(rows[0].subtitle, "main · 2 tareas");
        assert_eq!(rows[1].subtitle, "main · 0 tareas");
    }

    #[test]
    fn task_rows_are_sorted_by_recent_update_with_workspace_name() {
        let workspaces: Vec<WorkspaceOptionDto> = vec![workspace("w1", "uno")];
        let tasks: Vec<TaskDto> = vec![
            task("old", "w1", "2026-01-01T00:00:00Z"),
            task("new", "w1", "2026-02-01T00:00:00Z"),
            task("orphan", "missing", "2026-01-15T00:00:00Z"),
        ];
        let rows: Vec<SidebarTaskRow> = build_task_rows(&tasks, &workspaces);
        let ids: Vec<&str> = rows
            .iter()
            .map(|r: &SidebarTaskRow| r.id.as_str())
            .collect();
        assert_eq!(ids, ["new", "orphan", "old"]);
        assert_eq!(rows[0].subtitle, "Esperando · uno");
        assert_eq!(rows[1].subtitle, "Esperando");
    }

    #[test]
    fn task_rows_only_include_tasks_needing_attention() {
        let workspaces: Vec<WorkspaceOptionDto> = vec![workspace("w1", "uno")];
        let tasks: Vec<TaskDto> = vec![
            attention_task("calm", "w1", "2026-03-01T00:00:00Z", false),
            attention_task("urgent", "w1", "2026-01-01T00:00:00Z", true),
        ];
        let rows: Vec<SidebarTaskRow> = build_task_rows(&tasks, &workspaces);
        assert_eq!(rows.len(), 1);
        assert_eq!(rows[0].id, "urgent");
    }

    #[test]
    fn task_path_points_to_the_detail_route() {
        assert_eq!(task_path("abc"), "/tasks/abc");
    }
}
