#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use chrono::{DateTime, SubsecRound, Utc};
use pi_workflow::modules::tasks::domain::models::agent_event::AgentEvent;
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
