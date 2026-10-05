#[path = "../../common/mod.rs"]
mod common;

use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::{Duration, Instant};

use chrono::{DateTime, SubsecRound, Utc};
use diesel::{Connection, RunQueryDsl, SqliteConnection};
use pi_workflow::modules::tasks::domain::errors::task_error::TaskError;
use pi_workflow::modules::tasks::domain::models::agent_command::AgentCommand;
use pi_workflow::modules::tasks::domain::models::agent_event::{
    AgentEvent, DialogKind, DialogRequest,
};
use pi_workflow::modules::tasks::domain::models::launch_options::LaunchOptions;
use pi_workflow::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use pi_workflow::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use pi_workflow::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use pi_workflow::modules::tasks::domain::models::task_priority::TaskPriority;
use pi_workflow::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use pi_workflow::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use pi_workflow::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use pi_workflow::modules::tasks::domain::ports::outbound::launch_settings_port::LaunchSettingsPort;
use pi_workflow::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use pi_workflow::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;
use pi_workflow::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::TaskRepositoryAdapter;
use pi_workflow::modules::tasks::infrastructure::di::tasks_context::TasksContext;
use pi_workflow::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use pi_workflow::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use pi_workflow::modules::workspaces::infrastructure::adapters::outbound::persistence::workspace_repository_adapter::WorkspaceRepositoryAdapter;

const EVENT_COUNT: usize = 50;
const LOCK_HOLD: Duration = Duration::from_millis(6500);
const MAX_ENQUEUE_TIME: Duration = Duration::from_secs(2);
const DRAIN_TIMEOUT: Duration = Duration::from_secs(30);
const POLL_INTERVAL: Duration = Duration::from_millis(100);

struct StubRuntime;

#[async_trait::async_trait]
impl AgentRuntimePort for StubRuntime {
    async fn start(
        &self,
        _task_id: TaskId,
        _cwd: &Path,
        _resume: bool,
        _options: &LaunchOptions,
        _sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError> {
        Err(TaskError::Runtime("stub".to_string()))
    }

    async fn send(&self, _task_id: TaskId, _command: AgentCommand) -> Result<(), TaskError> {
        Err(TaskError::AgentNotRunning)
    }

    async fn stop(&self, _task_id: TaskId) -> Result<(), TaskError> {
        Ok(())
    }

    async fn is_running(&self, _task_id: TaskId) -> bool {
        false
    }
}

struct StubLaunchSettings;

#[async_trait::async_trait]
impl LaunchSettingsPort for StubLaunchSettings {
    async fn prepare(&self, _cwd: &Path) -> Result<LaunchOptions, TaskError> {
        Ok(LaunchOptions::default())
    }
}

struct StubWorkspaces;

#[async_trait::async_trait]
impl WorkspaceLookupPort for StubWorkspaces {
    async fn find(&self, _workspace_id: &str) -> Result<Option<WorkspaceRef>, TaskError> {
        Ok(None)
    }

    async fn list(&self) -> Result<Vec<WorkspaceRef>, TaskError> {
        Ok(Vec::new())
    }
}

/// Evento número `i`: de cada cinco, uno abre un diálogo, otro resuelve el del
/// grupo par anterior y el resto son mensajes. Los diálogos de grupos impares
/// quedan pendientes.
fn event(i: usize) -> AgentEvent {
    match i % 5 {
        0 => AgentEvent::DialogRequested(DialogRequest {
            id: format!("dialog-{i}"),
            title: format!("diálogo {i}"),
            kind: DialogKind::Confirm { message: None },
            timeout_ms: None,
        }),
        1 if (i / 5).is_multiple_of(2) => AgentEvent::DialogResolved {
            dialog_id: format!("dialog-{}", i - 1),
        },
        _ => AgentEvent::UserMessage {
            text: format!("mensaje {i}"),
        },
    }
}

fn expected_pending() -> Vec<String> {
    (0..EVENT_COUNT)
        .filter(|i| i.is_multiple_of(5) && !(i / 5).is_multiple_of(2))
        .map(|i| format!("dialog-{i}"))
        .collect()
}

fn agent_events(events: Vec<TaskEvent>) -> Vec<(i64, AgentEvent)> {
    events
        .into_iter()
        .filter_map(|e| match (e.seq, e.kind) {
            (Some(seq), TaskEventKind::Agent { event }) => Some((seq, event)),
            _ => None,
        })
        .collect()
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn no_agent_event_is_lost_while_sqlite_stays_locked_past_busy_timeout() {
    let db: common::TestDb = common::test_db().await;
    let now: DateTime<Utc> = Utc::now().trunc_subsecs(6);
    let workspace: Workspace = Workspace {
        id: WorkspaceId::new(),
        name: "demo".to_string(),
        path: PathBuf::from("/repos/demo"),
        base_branch: "main".to_string(),
        created_at: now,
    };
    WorkspaceRepositoryAdapter::new(db.pool.clone())
        .save(&workspace)
        .await
        .expect("workspace");
    let task: Task = Task {
        id: TaskId::new(),
        workspace_id: workspace.id.to_string(),
        title: "viva".to_string(),
        prompt: "haz algo".to_string(),
        branch: "pi/viva".to_string(),
        base_branch: "main".to_string(),
        worktree_path: PathBuf::from("/wt/viva"),
        status: TaskStatus::Running,
        priority: TaskPriority::default(),
        error: None,
        pending_dialogs: Vec::new(),
        subagents: SubagentActivity::default(),
        created_at: now,
        updated_at: now,
    };
    let repository: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    repository.insert(&task).await.expect("insert");

    let ctx: TasksContext = TasksContext::with_runtime(
        db.pool.clone(),
        db.dir.path().join("worktrees"),
        Arc::new(StubWorkspaces),
        Arc::new(StubRuntime),
        Arc::new(StubLaunchSettings),
    );

    // Segunda conexión que retiene el lock de escritura más que `busy_timeout`.
    let db_path: String = db.dir.path().join("test.db").to_string_lossy().into_owned();
    let (locked_tx, locked_rx): (std::sync::mpsc::Sender<()>, std::sync::mpsc::Receiver<()>) =
        std::sync::mpsc::channel();
    let locker: std::thread::JoinHandle<()> = std::thread::spawn(move || {
        let mut conn: SqliteConnection = SqliteConnection::establish(&db_path).expect("conexión");
        diesel::sql_query("BEGIN IMMEDIATE")
            .execute(&mut conn)
            .expect("begin");
        locked_tx.send(()).expect("señal");
        std::thread::sleep(LOCK_HOLD);
        diesel::sql_query("COMMIT")
            .execute(&mut conn)
            .expect("commit");
    });
    locked_rx
        .recv_timeout(Duration::from_secs(10))
        .expect("lock adquirido");

    let started: Instant = Instant::now();
    for i in 0..EVENT_COUNT {
        ctx.event_sink.handle(task.id, event(i)).await;
    }
    let enqueue_time: Duration = started.elapsed();
    assert!(
        enqueue_time < MAX_ENQUEUE_TIME,
        "handle bloqueó al lector durante {enqueue_time:?}"
    );

    let deadline: Instant = Instant::now() + DRAIN_TIMEOUT;
    let persisted: Vec<(i64, AgentEvent)> = loop {
        let current: Vec<(i64, AgentEvent)> =
            agent_events(repository.events(task.id, None).await.expect("events"));
        if current.len() >= EVENT_COUNT || Instant::now() >= deadline {
            break current;
        }
        tokio::time::sleep(POLL_INTERVAL).await;
    };
    locker.join().expect("hilo del lock");

    let expected: Vec<AgentEvent> = (0..EVENT_COUNT).map(event).collect();
    let actual: Vec<AgentEvent> = persisted.iter().map(|(_, e)| e.clone()).collect();
    assert_eq!(
        actual, expected,
        "eventos perdidos, duplicados o desordenados"
    );
    assert!(
        persisted.windows(2).all(|w| w[0].0 < w[1].0),
        "seq no es estrictamente creciente"
    );

    let stored: Task = repository
        .find(task.id)
        .await
        .expect("find")
        .expect("existe");
    let mut pending: Vec<String> = stored.pending_dialogs.into_iter().map(|d| d.id).collect();
    pending.sort();
    let mut want: Vec<String> = expected_pending();
    want.sort();
    assert_eq!(pending, want, "diálogos pendientes inconsistentes");
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn flush_events_persists_everything_accepted_before_returning() {
    let db: common::TestDb = common::test_db().await;
    let now: DateTime<Utc> = Utc::now().trunc_subsecs(6);
    let workspace: Workspace = Workspace {
        id: WorkspaceId::new(),
        name: "demo".to_string(),
        path: PathBuf::from("/repos/demo"),
        base_branch: "main".to_string(),
        created_at: now,
    };
    WorkspaceRepositoryAdapter::new(db.pool.clone())
        .save(&workspace)
        .await
        .expect("workspace");
    let task: Task = Task {
        id: TaskId::new(),
        workspace_id: workspace.id.to_string(),
        title: "viva".to_string(),
        prompt: "haz algo".to_string(),
        branch: "pi/viva".to_string(),
        base_branch: "main".to_string(),
        worktree_path: PathBuf::from("/wt/viva"),
        status: TaskStatus::Running,
        priority: TaskPriority::default(),
        error: None,
        pending_dialogs: Vec::new(),
        subagents: SubagentActivity::default(),
        created_at: now,
        updated_at: now,
    };
    let repository: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    repository.insert(&task).await.expect("insert");

    let ctx: TasksContext = TasksContext::with_runtime(
        db.pool.clone(),
        db.dir.path().join("worktrees"),
        Arc::new(StubWorkspaces),
        Arc::new(StubRuntime),
        Arc::new(StubLaunchSettings),
    );

    for i in 0..EVENT_COUNT {
        ctx.event_sink.handle(task.id, event(i)).await;
    }
    ctx.flush_events().await;

    let persisted: Vec<(i64, AgentEvent)> =
        agent_events(repository.events(task.id, None).await.expect("events"));
    let actual: Vec<AgentEvent> = persisted.into_iter().map(|(_, e)| e).collect();
    let expected: Vec<AgentEvent> = (0..EVENT_COUNT).map(event).collect();
    assert_eq!(actual, expected, "flush devolvió con eventos sin persistir");
}
