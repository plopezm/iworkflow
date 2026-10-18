use std::collections::HashMap;

use crate::core::dock::context::{DockContext, use_dock};
use leptos::prelude::*;

use crate::core::components::browser_notification::{
    request_permission_on_first_interaction, show_notification,
};
use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::presentation::state::task_notifications::{
    NotificationPlan, TaskNotification, TaskSnapshot, plan_notifications,
};
use crate::modules::tasks::presentation::state::task_store::{TaskStore, use_task_store};

const ACTIVE_COUNT: Msg = Msg {
    en: "{branch} · {count} active",
    es: "{branch} · {count} activas",
    zh: "{branch} · {count} 个进行中",
    de: "{branch} · {count} aktiv",
    fr: "{branch} · {count} actives",
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
                subtitle: ACTIVE_COUNT
                    .fmt(&[("branch", &w.base_branch), ("count", &count.to_string())]),
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
/// `Suspend`) derivados del `TaskStore` compartido, que se mantiene al día con SSE.
#[derive(Clone, Copy)]
pub struct AppSidebarState {
    pub workspace_rows: RwSignal<Vec<SidebarWorkspaceRow>>,
    pub task_rows: RwSignal<Vec<SidebarTaskRow>>,
    dock: DockContext,
}

impl AppSidebarState {
    pub fn new() -> Self {
        let store: TaskStore = use_task_store();
        let dock: DockContext = use_dock();
        let workspace_rows: RwSignal<Vec<SidebarWorkspaceRow>> = RwSignal::new(Vec::new());
        let task_rows: RwSignal<Vec<SidebarTaskRow>> = RwSignal::new(Vec::new());
        // Client-only, after hydration: the browser shows its native prompt on first click.
        Effect::new(move |_| request_permission_on_first_interaction());

        // Publica desde un `Effect` (solo cliente, tras hidratar) y conserva el valor
        // anterior mientras se recarga.
        Effect::new(move |last: Option<Option<HashMap<String, TaskSnapshot>>>| {
            let previous: Option<HashMap<String, TaskSnapshot>> = last.flatten();
            let loaded_workspaces: Option<Result<Vec<WorkspaceOptionDto>, ServerFnError>> =
                store.workspaces.get();
            let tasks: Vec<TaskDto> = store.active.get();
            if let (Some(Ok(workspaces)), true) = (loaded_workspaces, store.loaded.get()) {
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
                let plan: NotificationPlan = plan_notifications(previous.as_ref(), &tasks);
                notify_transitions(&plan.notifications);
                return Some(plan.next_baseline);
            }
            previous
        });

        Self {
            workspace_rows,
            task_rows,
            dock,
        }
    }

    /// Indica si `task_id` es la tarea visible en alguna pestaña activa.
    pub fn is_task_selected(&self, task_id: &str) -> bool {
        self.dock.is_path_visible(&task_path(task_id))
    }

    /// Indica si `path` (el tablero de un workspace) es visible en alguna pestaña activa.
    pub fn is_path_visible(&self, path: &str) -> bool {
        self.dock.is_path_visible(path)
    }
}

/// Raises a native notification for every task that just finished.
fn notify_transitions(notifications: &[TaskNotification]) {
    for notification in notifications {
        show_notification(
            &notification.title,
            &notification.body,
            &notification.task_id,
        );
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
            memory_limit_mb: None,
            cpu_limit_percent: None,
            error: None,
            resume_at: None,
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
        assert_eq!(rows[0].subtitle, "main · 2 active");
        assert_eq!(rows[1].subtitle, "main · 0 active");
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
        assert_eq!(rows[0].subtitle, "Waiting · uno");
        assert_eq!(rows[1].subtitle, "Waiting");
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
