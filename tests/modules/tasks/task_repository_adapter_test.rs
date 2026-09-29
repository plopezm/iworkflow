#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use chrono::{DateTime, SubsecRound, Utc};
use pi_workflow::modules::tasks::domain::models::agent_event::AgentEvent;
use pi_workflow::modules::tasks::domain::models::pending_dialog::PendingDialog;
use pi_workflow::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use pi_workflow::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use pi_workflow::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use pi_workflow::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::TaskRepositoryAdapter;
use pi_workflow::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use pi_workflow::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use pi_workflow::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use pi_workflow::modules::workspaces::infrastructure::adapters::outbound::persistence::workspace_repository_adapter::WorkspaceRepositoryAdapter;

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
        error: None,
        pending_dialogs: Vec::new(),
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
        )
        .await
        .expect("update");
    let rejected: bool = adapter
        .update_details_if_backlog(started.id, "X".to_string(), "Y".to_string())
        .await
        .expect("update");
    let missing: bool = adapter
        .update_details_if_backlog(TaskId::new(), "X".to_string(), "Y".to_string())
        .await
        .expect("update");

    assert!(updated);
    assert!(!rejected);
    assert!(!missing);
    let edited: Task = found(&adapter, backlog.id).await;
    assert_eq!(edited.title, "Nuevo título");
    assert_eq!(edited.prompt, "Nuevo prompt");
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
