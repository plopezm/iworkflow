//! Fixtures compartidos por los tests de los casos de uso de tareas.

use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use chrono::{DateTime, Utc};

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::resource_limits::ResourceLimits;
use crate::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::models::task_priority::TaskPriority;
use crate::modules::tasks::domain::models::usage_limit::UsageLimitState;
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::mocks::{
    MockTaskEventPublisher, MockTaskRepository, MockWorkspaceLookup,
};

/// Transiciones registradas: estado y error.
pub type StatusLog = Arc<Mutex<Vec<(TaskStatus, Option<String>)>>>;

pub const REPO: &str = "/repos/demo";
pub const WORKTREE: &str = "/worktrees/demo/abc-task";

pub fn task(status: TaskStatus) -> Task {
    task_with_pending_dialogs(status, Vec::new())
}

/// Como [`task`], con diálogos pendientes (sin expiración) para los `ids` dados.
pub fn task_with_pending_dialogs(status: TaskStatus, ids: Vec<String>) -> Task {
    let pending_dialogs: Vec<PendingDialog> = ids
        .into_iter()
        .map(|id: String| PendingDialog::new(id, None))
        .collect();
    Task {
        id: TaskId::new(),
        workspace_id: "ws-1".to_string(),
        title: "Arregla el login".to_string(),
        prompt: "Arregla el login".to_string(),
        branch: "pi/arregla-el-login-abc".to_string(),
        base_branch: "main".to_string(),
        worktree_path: PathBuf::from(WORKTREE),
        status,
        priority: TaskPriority::default(),
        limits: ResourceLimits::default(),
        usage_limit: UsageLimitState::default(),
        error: None,
        pending_dialogs,
        subagents: Default::default(),
        diff_range: None,
        created_at: Utc::now(),
        updated_at: Utc::now(),
    }
}

pub fn workspace_ref() -> WorkspaceRef {
    WorkspaceRef {
        id: "ws-1".to_string(),
        name: "demo".to_string(),
        path: PathBuf::from(REPO),
        base_branch: "main".to_string(),
    }
}

/// Operación aplicada a los diálogos pendientes de una tarea, tal y como la vería el
/// repositorio (que ahora expone operaciones atómicas en lugar de reemplazar la lista).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PendingDialogOp {
    Added {
        dialog_id: String,
        expires_at: Option<DateTime<Utc>>,
    },
    Removed {
        dialog_id: String,
    },
    Cleared,
}

/// Registro de lo que ocurre en el repositorio simulado.
#[derive(Clone, Default)]
pub struct Recorded {
    pub statuses: StatusLog,
    pub events: Arc<Mutex<Vec<TaskEventKind>>>,
    pub published: Arc<Mutex<Vec<TaskEvent>>>,
    pub pending_dialog_ops: Arc<Mutex<Vec<PendingDialogOp>>>,
    pub subagent_activity: Arc<Mutex<Vec<SubagentActivity>>>,
}

impl Recorded {
    pub fn statuses(&self) -> Vec<TaskStatus> {
        self.statuses
            .lock()
            .expect("lock")
            .iter()
            .map(|(s, _)| *s)
            .collect()
    }

    pub fn last_error(&self) -> Option<String> {
        self.statuses
            .lock()
            .expect("lock")
            .last()
            .and_then(|(_, e)| e.clone())
    }

    pub fn subagent_activity(&self) -> Vec<SubagentActivity> {
        self.subagent_activity.lock().expect("lock").clone()
    }

    pub fn pending_dialog_ops(&self) -> Vec<PendingDialogOp> {
        self.pending_dialog_ops.lock().expect("lock").clone()
    }
}

/// Repositorio simulado que devuelve `task` y registra estados y eventos.
pub fn repository_with(task: Option<Task>, recorded: &Recorded) -> MockTaskRepository {
    let mut repository: MockTaskRepository = MockTaskRepository::new();
    repository
        .expect_find()
        .returning(move |_| Ok(task.clone()));
    record_into(repository, recorded)
}

/// Como [`repository_with`], pero sin `find`: el llamante define cómo se resuelve.
pub fn repository_without_find(recorded: &Recorded) -> MockTaskRepository {
    record_into(MockTaskRepository::new(), recorded)
}

fn record_into(mut repository: MockTaskRepository, recorded: &Recorded) -> MockTaskRepository {
    let statuses: StatusLog = recorded.statuses.clone();
    repository
        .expect_update_status()
        .returning(move |_, status, error| {
            statuses.lock().expect("lock").push((status, error));
            Ok(())
        });
    let events: Arc<Mutex<Vec<TaskEventKind>>> = recorded.events.clone();
    repository.expect_append_event().returning(move |id, kind| {
        events.lock().expect("lock").push(kind.clone());
        Ok(TaskEvent {
            task_id: id,
            seq: Some(1),
            kind,
            created_at: Utc::now(),
        })
    });
    let added: Arc<Mutex<Vec<PendingDialogOp>>> = recorded.pending_dialog_ops.clone();
    repository
        .expect_add_pending_dialog()
        .returning(move |_, dialog: PendingDialog| {
            added.lock().expect("lock").push(PendingDialogOp::Added {
                dialog_id: dialog.id,
                expires_at: dialog.expires_at,
            });
            Ok(())
        });
    let removed: Arc<Mutex<Vec<PendingDialogOp>>> = recorded.pending_dialog_ops.clone();
    repository
        .expect_remove_pending_dialog()
        .returning(move |_, dialog_id: String| {
            removed
                .lock()
                .expect("lock")
                .push(PendingDialogOp::Removed { dialog_id });
            Ok(())
        });
    let cleared: Arc<Mutex<Vec<PendingDialogOp>>> = recorded.pending_dialog_ops.clone();
    repository
        .expect_clear_pending_dialogs()
        .returning(move |_| {
            cleared.lock().expect("lock").push(PendingDialogOp::Cleared);
            Ok(())
        });
    let activity: Arc<Mutex<Vec<SubagentActivity>>> = recorded.subagent_activity.clone();
    repository
        .expect_set_subagent_activity()
        .returning(move |_, value: SubagentActivity| {
            activity.lock().expect("lock").push(value);
            Ok(())
        });
    repository
}

pub fn publisher(recorded: &Recorded) -> MockTaskEventPublisher {
    let mut publisher: MockTaskEventPublisher = MockTaskEventPublisher::new();
    let published: Arc<Mutex<Vec<TaskEvent>>> = recorded.published.clone();
    publisher
        .expect_publish()
        .returning(move |event| published.lock().expect("lock").push(event));
    publisher
}

pub fn workspaces() -> MockWorkspaceLookup {
    let mut lookup: MockWorkspaceLookup = MockWorkspaceLookup::new();
    lookup
        .expect_find()
        .returning(|_| Ok(Some(workspace_ref())));
    lookup
}

pub fn lifecycle(task: Option<Task>, recorded: &Recorded) -> Arc<TaskLifecycle> {
    lifecycle_with(repository_with(task, recorded), recorded)
}

/// Como `lifecycle`, pero recibiendo un repositorio ya configurado (para añadirle
/// expectativas adicionales, p.ej. `update_plan` o `delete`).
pub fn lifecycle_with(repository: MockTaskRepository, recorded: &Recorded) -> Arc<TaskLifecycle> {
    Arc::new(TaskLifecycle::new(
        Arc::new(repository),
        Arc::new(publisher(recorded)),
        Arc::new(workspaces()),
    ))
}
