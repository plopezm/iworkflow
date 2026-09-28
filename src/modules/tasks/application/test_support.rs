//! Fixtures compartidos por los tests de los casos de uso de tareas.

use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use chrono::Utc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::mocks::{
    MockTaskEventPublisher, MockTaskRepository, MockWorkspaceLookup,
};

/// Transiciones registradas: estado y error.
pub type StatusLog = Arc<Mutex<Vec<(TaskStatus, Option<String>)>>>;

pub const REPO: &str = "/repos/demo";
pub const WORKTREE: &str = "/worktrees/demo/abc-task";

pub fn task(status: TaskStatus) -> Task {
    Task {
        id: TaskId::new(),
        workspace_id: "ws-1".to_string(),
        title: "Arregla el login".to_string(),
        prompt: "Arregla el login".to_string(),
        branch: "pi/arregla-el-login-abc".to_string(),
        base_branch: "main".to_string(),
        worktree_path: PathBuf::from(WORKTREE),
        status,
        error: None,
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

/// Registro de lo que ocurre en el repositorio simulado.
#[derive(Clone, Default)]
pub struct Recorded {
    pub statuses: StatusLog,
    pub events: Arc<Mutex<Vec<TaskEventKind>>>,
    pub published: Arc<Mutex<Vec<TaskEvent>>>,
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
}

/// Repositorio simulado que devuelve `task` y registra estados y eventos.
pub fn repository_with(task: Option<Task>, recorded: &Recorded) -> MockTaskRepository {
    let mut repository: MockTaskRepository = MockTaskRepository::new();
    repository
        .expect_find()
        .returning(move |_| Ok(task.clone()));
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
    Arc::new(TaskLifecycle::new(
        Arc::new(repository_with(task, recorded)),
        Arc::new(publisher(recorded)),
        Arc::new(workspaces()),
    ))
}
