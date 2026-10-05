#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use chrono::{DateTime, SubsecRound, Utc};
use iworkflow::modules::tasks::domain::models::agent_event::AgentEvent;
use iworkflow::modules::tasks::domain::models::pending_dialog::PendingDialog;
use iworkflow::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use iworkflow::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use iworkflow::modules::tasks::domain::models::task_priority::TaskPriority;
use iworkflow::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use iworkflow::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use iworkflow::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::TaskRepositoryAdapter;
use iworkflow::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use iworkflow::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use iworkflow::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use iworkflow::modules::workspaces::infrastructure::adapters::outbound::persistence::workspace_repository_adapter::WorkspaceRepositoryAdapter;

async fn workspace(db: &common::TestDb, path: &str) -> Workspace {
    let workspace: Workspace = Workspace {
        id: WorkspaceId::new(),
        name: "demo".to_string(),
        path: PathBuf::from(path),
        base_branch: "main".to_string(),
        created_at: Utc::now(),
    };
    WorkspaceRepositoryAdapter::new(db.pool.clone())
        .save(&workspace)
        .await
        .expect("workspace");
    workspace
}

fn task(workspace_id: &str, title: &str) -> Task {
    let now: DateTime<Utc> = Utc::now().trunc_subsecs(6);
    Task {
        id: TaskId::new(),
        workspace_id: workspace_id.to_string(),
        title: title.to_string(),
        prompt: "haz algo".to_string(),
        branch: format!("pi/{title}"),
        base_branch: "main".to_string(),
        worktree_path: PathBuf::from(format!("/wt/{title}")),
        status: TaskStatus::Starting,
        priority: TaskPriority::default(),
        error: None,
        pending_dialogs: Vec::new(),
        subagents: SubagentActivity::default(),
        created_at: now,
        updated_at: now,
    }
}

#[tokio::test(flavor = "multi_thread")]
async fn inserts_finds_and_updates_status() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let original: Task = task(&ws.id.to_string(), "uno");

    adapter.insert(&original).await.expect("insert");
    adapter
        .update_status(
            original.id,
            TaskStatus::Conflict,
            Some("conflictos".to_string()),
        )
        .await
        .expect("update");

    let found: Task = adapter
        .find(original.id)
        .await
        .expect("find")
        .expect("existe");
    assert_eq!(found.title, "uno");
    assert_eq!(found.status, TaskStatus::Conflict);
    assert_eq!(found.error.as_deref(), Some("conflictos"));
    assert!(found.updated_at >= original.updated_at);
}

#[tokio::test(flavor = "multi_thread")]
async fn lists_newest_first_filtered_by_workspace_and_status() {
    let db: common::TestDb = common::test_db().await;
    let a: Workspace = workspace(&db, "/repos/a").await;
    let b: Workspace = workspace(&db, "/repos/b").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let first: Task = task(&a.id.to_string(), "primera");
    let mut second: Task = task(&a.id.to_string(), "segunda");
    second.created_at = first.created_at + chrono::Duration::seconds(1);
    second.status = TaskStatus::Running;
    let other: Task = task(&b.id.to_string(), "otra");
    for t in [&first, &second, &other] {
        adapter.insert(t).await.expect("insert");
    }

    let of_a: Vec<String> = adapter
        .list(Some(a.id.to_string()))
        .await
        .expect("list")
        .into_iter()
        .map(|t: Task| t.title)
        .collect();
    let running: Vec<Task> = adapter
        .list_by_status(&[TaskStatus::Running, TaskStatus::Idle])
        .await
        .expect("list");

    assert_eq!(of_a, vec!["segunda", "primera"]);
    assert_eq!(adapter.list(None).await.expect("list").len(), 3);
    assert_eq!(running.len(), 1);
    assert_eq!(running[0].id, second.id);
}

#[tokio::test(flavor = "multi_thread")]
async fn appends_events_with_increasing_seq_and_reads_after_cursor() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let t: Task = task(&ws.id.to_string(), "uno");
    adapter.insert(&t).await.expect("insert");

    let first: TaskEvent = adapter
        .append_event(t.id, TaskEventKind::status(TaskStatus::Starting, None))
        .await
        .expect("append");
    let second: TaskEvent = adapter
        .append_event(
            t.id,
            TaskEventKind::Agent {
                event: AgentEvent::UserMessage {
                    text: "hola".into(),
                },
            },
        )
        .await
        .expect("append");

    let all: Vec<TaskEvent> = adapter.events(t.id, None).await.expect("events");
    let after_first: Vec<TaskEvent> = adapter.events(t.id, first.seq).await.expect("events");

    assert!(second.seq > first.seq);
    assert_eq!(all.len(), 2);
    assert_eq!(all[1].kind, second.kind);
    assert_eq!(after_first, vec![second]);
}

fn backlog_task(workspace_id: &str, title: &str) -> Task {
    Task {
        status: TaskStatus::Backlog,
        ..task(workspace_id, title)
    }
}

async fn found(adapter: &TaskRepositoryAdapter, id: TaskId) -> Task {
    adapter.find(id).await.expect("find").expect("existe")
}

#[tokio::test(flavor = "multi_thread")]
async fn updates_details_only_while_in_backlog() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let backlog: Task = backlog_task(&ws.id.to_string(), "uno");
    let started: Task = task(&ws.id.to_string(), "dos");
    adapter.insert(&backlog).await.expect("insert");
    adapter.insert(&started).await.expect("insert");

    let updated: bool = adapter
        .update_details_if_backlog(
            backlog.id,
            "Nuevo título".to_string(),
            "Nuevo prompt".to_string(),
            TaskPriority::High,
        )
        .await
        .expect("update");
    let rejected: bool = adapter
        .update_details_if_backlog(
            started.id,
            "X".to_string(),
            "Y".to_string(),
            TaskPriority::Urgent,
        )
        .await
        .expect("update");
    let missing: bool = adapter
        .update_details_if_backlog(
            TaskId::new(),
            "X".to_string(),
            "Y".to_string(),
            TaskPriority::Urgent,
        )
        .await
        .expect("update");

    assert!(updated);
    assert!(!rejected);
    assert!(!missing);
    let edited: Task = found(&adapter, backlog.id).await;
    assert_eq!(edited.title, "Nuevo título");
    assert_eq!(edited.prompt, "Nuevo prompt");
    assert_eq!(edited.priority, TaskPriority::High);
    assert_eq!(
        found(&adapter, started.id).await.priority,
        TaskPriority::Normal
    );
    assert!(edited.updated_at >= backlog.updated_at);
    assert_eq!(found(&adapter, started.id).await.title, "dos");
}

#[tokio::test(flavor = "multi_thread")]
async fn claim_for_start_sets_plan_and_is_exclusive() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let original: Task = backlog_task(&ws.id.to_string(), "uno");
    adapter.insert(&original).await.expect("insert");

    let first: bool = adapter
        .claim_for_start(
            original.id,
            "pi/nueva-rama".to_string(),
            PathBuf::from("/wt/nueva"),
            "develop".to_string(),
        )
        .await
        .expect("claim");
    let second: bool = adapter
        .claim_for_start(
            original.id,
            "pi/otra".to_string(),
            PathBuf::from("/wt/otra"),
            "main".to_string(),
        )
        .await
        .expect("claim");

    assert!(first);
    assert!(!second);
    let claimed: Task = found(&adapter, original.id).await;
    assert_eq!(claimed.status, TaskStatus::Starting);
    assert_eq!(claimed.branch, "pi/nueva-rama");
    assert_eq!(claimed.worktree_path, PathBuf::from("/wt/nueva"));
    assert_eq!(claimed.base_branch, "develop");
}

#[tokio::test(flavor = "multi_thread")]
async fn claim_for_start_rejects_missing_task() {
    let db: common::TestDb = common::test_db().await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());

    let claimed: bool = adapter
        .claim_for_start(
            TaskId::new(),
            "b".to_string(),
            PathBuf::from("/wt/b"),
            "main".to_string(),
        )
        .await
        .expect("claim");

    assert!(!claimed);
}

#[tokio::test(flavor = "multi_thread")]
async fn revert_start_claim_returns_starting_task_to_backlog() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let original: Task = backlog_task(&ws.id.to_string(), "uno");
    adapter.insert(&original).await.expect("insert");
    adapter
        .claim_for_start(
            original.id,
            "pi/x".to_string(),
            PathBuf::from("/wt/x"),
            "main".to_string(),
        )
        .await
        .expect("claim");

    let reverted: bool = adapter
        .revert_start_claim(original.id)
        .await
        .expect("revert");
    let again: bool = adapter
        .revert_start_claim(original.id)
        .await
        .expect("revert");

    assert!(reverted);
    assert!(!again);
    assert_eq!(
        found(&adapter, original.id).await.status,
        TaskStatus::Backlog
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn revert_start_claim_ignores_tasks_not_in_starting() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let running: Task = Task {
        status: TaskStatus::Running,
        ..task(&ws.id.to_string(), "uno")
    };
    adapter.insert(&running).await.expect("insert");

    let reverted: bool = adapter
        .revert_start_claim(running.id)
        .await
        .expect("revert");

    assert!(!reverted);
    assert_eq!(
        found(&adapter, running.id).await.status,
        TaskStatus::Running
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn pending_dialogs_add_remove_clear_and_ignore_duplicates() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let original: Task = task(&ws.id.to_string(), "uno");
    adapter.insert(&original).await.expect("insert");
    assert!(original.pending_dialogs.is_empty());
    let d1: PendingDialog = PendingDialog::new("d1".to_string(), None);
    let d2: PendingDialog = PendingDialog::new("d2".to_string(), None);

    adapter
        .add_pending_dialog(original.id, d1.clone())
        .await
        .expect("add");
    adapter
        .add_pending_dialog(original.id, d1.clone())
        .await
        .expect("add dup");
    adapter
        .add_pending_dialog(original.id, d2.clone())
        .await
        .expect("add");

    assert_eq!(
        found(&adapter, original.id).await.pending_dialogs,
        vec![d1, d2.clone()]
    );

    adapter
        .remove_pending_dialog(original.id, "d1".to_string())
        .await
        .expect("remove");
    assert_eq!(found(&adapter, original.id).await.pending_dialogs, vec![d2]);

    adapter
        .clear_pending_dialogs(original.id)
        .await
        .expect("clear");
    assert!(
        found(&adapter, original.id)
            .await
            .pending_dialogs
            .is_empty()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn add_pending_dialog_ignored_for_missing_or_dead_task() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let merged: Task = Task {
        status: TaskStatus::Merged,
        ..task(&ws.id.to_string(), "uno")
    };
    adapter.insert(&merged).await.expect("insert");
    let dialog: PendingDialog = PendingDialog::new("d1".to_string(), None);

    adapter
        .add_pending_dialog(merged.id, dialog.clone())
        .await
        .expect("add");
    adapter
        .add_pending_dialog(TaskId::new(), dialog)
        .await
        .expect("add missing");

    assert!(found(&adapter, merged.id).await.pending_dialogs.is_empty());
}

#[tokio::test(flavor = "multi_thread")]
async fn deletes_backlog_task_and_cascades_events() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let original: Task = backlog_task(&ws.id.to_string(), "uno");
    adapter.insert(&original).await.expect("insert");
    adapter
        .append_event(
            original.id,
            TaskEventKind::status(TaskStatus::Backlog, None),
        )
        .await
        .expect("append");

    let deleted: bool = adapter
        .delete_if_backlog(original.id)
        .await
        .expect("delete");

    assert!(deleted);
    assert_eq!(adapter.find(original.id).await.expect("find"), None);
    assert_eq!(
        adapter.events(original.id, None).await.expect("events"),
        Vec::new()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn delete_if_backlog_keeps_tasks_in_other_states() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let started: Task = task(&ws.id.to_string(), "uno");
    adapter.insert(&started).await.expect("insert");

    let deleted: bool = adapter.delete_if_backlog(started.id).await.expect("delete");
    let missing: bool = adapter
        .delete_if_backlog(TaskId::new())
        .await
        .expect("delete");

    assert!(!deleted);
    assert!(!missing);
    assert!(adapter.find(started.id).await.expect("find").is_some());
}

#[tokio::test(flavor = "multi_thread")]
async fn delete_inactive_by_workspace_deletes_all_inactive_tasks_and_their_events() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let backlog: Task = backlog_task(&ws.id.to_string(), "uno");
    let merged: Task = Task {
        status: TaskStatus::Merged,
        ..task(&ws.id.to_string(), "dos")
    };
    adapter.insert(&backlog).await.expect("insert");
    adapter.insert(&merged).await.expect("insert");
    adapter
        .append_event(backlog.id, TaskEventKind::status(TaskStatus::Backlog, None))
        .await
        .expect("append");

    let deleted: Option<Vec<TaskId>> = adapter
        .delete_inactive_by_workspace(ws.id.to_string())
        .await
        .expect("delete");

    let mut ids: Vec<TaskId> = deleted.expect("borrado");
    ids.sort_by_key(|id: &TaskId| id.to_string());
    let mut expected: Vec<TaskId> = vec![backlog.id, merged.id];
    expected.sort_by_key(|id: &TaskId| id.to_string());
    assert_eq!(ids, expected);
    assert_eq!(adapter.find(backlog.id).await.expect("find"), None);
    assert_eq!(adapter.find(merged.id).await.expect("find"), None);
    assert_eq!(
        adapter.events(backlog.id, None).await.expect("events"),
        Vec::new()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn delete_inactive_by_workspace_returns_none_and_deletes_nothing_when_one_task_is_active() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let backlog: Task = backlog_task(&ws.id.to_string(), "uno");
    let running: Task = task(&ws.id.to_string(), "dos");
    adapter.insert(&backlog).await.expect("insert");
    adapter.insert(&running).await.expect("insert");

    let deleted: Option<Vec<TaskId>> = adapter
        .delete_inactive_by_workspace(ws.id.to_string())
        .await
        .expect("delete");

    assert_eq!(deleted, None);
    assert!(adapter.find(backlog.id).await.expect("find").is_some());
    assert!(adapter.find(running.id).await.expect("find").is_some());
}

#[tokio::test(flavor = "multi_thread")]
async fn delete_inactive_by_workspace_does_not_touch_other_workspaces() {
    let db: common::TestDb = common::test_db().await;
    let a: Workspace = workspace(&db, "/repos/a").await;
    let b: Workspace = workspace(&db, "/repos/b").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let backlog_a: Task = backlog_task(&a.id.to_string(), "uno");
    let backlog_b: Task = backlog_task(&b.id.to_string(), "dos");
    adapter.insert(&backlog_a).await.expect("insert");
    adapter.insert(&backlog_b).await.expect("insert");

    let deleted: Option<Vec<TaskId>> = adapter
        .delete_inactive_by_workspace(a.id.to_string())
        .await
        .expect("delete");

    assert_eq!(deleted, Some(vec![backlog_a.id]));
    assert_eq!(adapter.find(backlog_a.id).await.expect("find"), None);
    assert!(adapter.find(backlog_b.id).await.expect("find").is_some());
}

#[tokio::test(flavor = "multi_thread")]
async fn workspace_with_tasks_cannot_be_deleted() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    TaskRepositoryAdapter::new(db.pool.clone())
        .insert(&task(&ws.id.to_string(), "uno"))
        .await
        .expect("insert");

    let result: Result<(), WorkspaceError> = WorkspaceRepositoryAdapter::new(db.pool.clone())
        .delete(ws.id)
        .await;

    assert_eq!(result, Err(WorkspaceError::InUse));
}

#[tokio::test(flavor = "multi_thread")]
async fn subagent_activity_persists_while_live_and_resets_when_agent_is_gone() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/subagents").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let mut live: Task = task(&ws.id.to_string(), "vivo");
    live.status = TaskStatus::Idle;
    adapter.insert(&live).await.expect("insert");

    adapter
        .set_subagent_activity(live.id, SubagentActivity::new(2, 1))
        .await
        .expect("actividad");
    assert_eq!(
        found(&adapter, live.id).await.subagents,
        SubagentActivity::new(2, 1)
    );

    adapter
        .update_status(live.id, TaskStatus::AwaitingReview, None)
        .await
        .expect("estado");
    assert_eq!(
        found(&adapter, live.id).await.subagents,
        SubagentActivity::default()
    );

    adapter
        .set_subagent_activity(live.id, SubagentActivity::new(3, 0))
        .await
        .expect("ignorada");
    assert_eq!(
        found(&adapter, live.id).await.subagents,
        SubagentActivity::default()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn priority_persists_on_insert_and_defaults_to_normal() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/a").await;
    let adapter: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    let urgent: Task = Task {
        priority: TaskPriority::Urgent,
        ..backlog_task(&ws.id.to_string(), "uno")
    };
    let normal: Task = backlog_task(&ws.id.to_string(), "dos");
    adapter.insert(&urgent).await.expect("insert");
    adapter.insert(&normal).await.expect("insert");

    assert_eq!(
        found(&adapter, urgent.id).await.priority,
        TaskPriority::Urgent
    );
    assert_eq!(
        found(&adapter, normal.id).await.priority,
        TaskPriority::Normal
    );
}

const CONCURRENT_TASKS: usize = 6;
const OPS_PER_TASK: usize = 25;

/// Mezcla escrituras concurrentes (transacciones read-modify-write y escrituras simples)
/// sobre las mismas filas. Ninguna debe fallar con `database is locked` ni perder datos.
#[tokio::test(flavor = "multi_thread", worker_threads = 8)]
async fn concurrent_writers_never_fail_nor_lose_updates() {
    let db: common::TestDb = common::test_db().await;
    let ws: Workspace = workspace(&db, "/repos/concurrent").await;
    let adapter: std::sync::Arc<TaskRepositoryAdapter> =
        std::sync::Arc::new(TaskRepositoryAdapter::new(db.pool.clone()));

    let mut tasks: Vec<Task> = Vec::new();
    for index in 0..CONCURRENT_TASKS {
        let t: Task = task(&ws.id.to_string(), &format!("conc{index}"));
        adapter.insert(&t).await.expect("insert");
        adapter
            .update_status(t.id, TaskStatus::Running, None)
            .await
            .expect("running");
        tasks.push(t);
    }

    let mut handles: Vec<tokio::task::JoinHandle<Result<(), String>>> = Vec::new();
    for t in &tasks {
        for op in 0..OPS_PER_TASK {
            let id: TaskId = t.id;
            let repo: std::sync::Arc<TaskRepositoryAdapter> = adapter.clone();
            handles.push(tokio::spawn(async move {
                let dialog_id: String = format!("d{op}");
                repo.add_pending_dialog(id, PendingDialog::new(dialog_id.clone(), None))
                    .await
                    .map_err(|e| format!("add: {e}"))?;
                repo.append_event(
                    id,
                    TaskEventKind::Agent {
                        event: AgentEvent::UserMessage {
                            text: format!("m{op}"),
                        },
                    },
                )
                .await
                .map_err(|e| format!("append: {e}"))?;
                repo.set_subagent_activity(id, SubagentActivity::new(1, 0))
                    .await
                    .map_err(|e| format!("subagents: {e}"))?;
                // Los pares pares se resuelven; los impares quedan pendientes.
                if op % 2 == 0 {
                    repo.remove_pending_dialog(id, dialog_id)
                        .await
                        .map_err(|e| format!("remove: {e}"))?;
                }
                Ok(())
            }));
        }
    }

    let mut failures: Vec<String> = Vec::new();
    for handle in handles {
        if let Err(message) = handle.await.expect("join") {
            failures.push(message);
        }
    }
    assert!(failures.is_empty(), "escrituras fallidas: {failures:?}");

    for t in &tasks {
        let found: Task = adapter.find(t.id).await.expect("find").expect("existe");
        let mut ids: Vec<String> = found
            .pending_dialogs
            .iter()
            .map(|d: &PendingDialog| d.id.clone())
            .collect();
        ids.sort();
        let mut expected: Vec<String> = (0..OPS_PER_TASK)
            .filter(|op: &usize| op % 2 == 1)
            .map(|op: usize| format!("d{op}"))
            .collect();
        expected.sort();
        assert_eq!(ids, expected, "se perdió una actualización de diálogos");

        let events: Vec<TaskEvent> = adapter.events(t.id, None).await.expect("events");
        assert_eq!(events.len(), OPS_PER_TASK, "faltan eventos");
    }
}

/// El borrado transaccional de un workspace no debe fallar mientras otras tareas escriben.
#[tokio::test(flavor = "multi_thread", worker_threads = 8)]
async fn delete_inactive_by_workspace_survives_concurrent_writes() {
    let db: common::TestDb = common::test_db().await;
    let target: Workspace = workspace(&db, "/repos/target").await;
    let busy: Workspace = workspace(&db, "/repos/busy").await;
    let adapter: std::sync::Arc<TaskRepositoryAdapter> =
        std::sync::Arc::new(TaskRepositoryAdapter::new(db.pool.clone()));

    let live: Task = task(&busy.id.to_string(), "live");
    adapter.insert(&live).await.expect("insert");
    adapter
        .update_status(live.id, TaskStatus::Running, None)
        .await
        .expect("running");

    let mut handles: Vec<tokio::task::JoinHandle<Result<(), String>>> = Vec::new();
    for op in 0..60 {
        let repo: std::sync::Arc<TaskRepositoryAdapter> = adapter.clone();
        let id: TaskId = live.id;
        let target_id: String = target.id.to_string();
        handles.push(tokio::spawn(async move {
            if op % 6 == 0 {
                repo.delete_inactive_by_workspace(target_id)
                    .await
                    .map(|_| ())
                    .map_err(|e| format!("delete: {e}"))
            } else {
                repo.add_pending_dialog(id, PendingDialog::new(format!("x{op}"), None))
                    .await
                    .map_err(|e| format!("add: {e}"))
            }
        }));
    }
    let mut failures: Vec<String> = Vec::new();
    for handle in handles {
        if let Err(message) = handle.await.expect("join") {
            failures.push(message);
        }
    }
    assert!(failures.is_empty(), "escrituras fallidas: {failures:?}");
}
