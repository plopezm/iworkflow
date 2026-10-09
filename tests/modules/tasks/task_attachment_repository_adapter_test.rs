#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use chrono::{DateTime, SubsecRound, Utc};
use iworkflow::modules::tasks::domain::errors::task_error::TaskError;
use iworkflow::modules::tasks::domain::models::stored_attachment::StoredAttachment;
use iworkflow::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use iworkflow::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use iworkflow::modules::tasks::domain::models::task_priority::TaskPriority;
use iworkflow::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
use iworkflow::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use iworkflow::modules::tasks::infrastructure::adapters::outbound::persistence::task_attachment_repository_adapter::TaskAttachmentRepositoryAdapter;
use iworkflow::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::TaskRepositoryAdapter;
use iworkflow::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use iworkflow::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use iworkflow::modules::workspaces::infrastructure::adapters::outbound::persistence::workspace_repository_adapter::WorkspaceRepositoryAdapter;

async fn backlog_task(db: &common::TestDb, title: &str) -> Task {
    let workspace: Workspace = Workspace {
        id: WorkspaceId::new(),
        name: "demo".to_string(),
        path: PathBuf::from(format!("/repos/{title}")),
        base_branch: "main".to_string(),
        created_at: Utc::now(),
    };
    WorkspaceRepositoryAdapter::new(db.pool.clone())
        .save(&workspace)
        .await
        .expect("workspace");
    let now: DateTime<Utc> = Utc::now().trunc_subsecs(6);
    let task: Task = Task {
        id: TaskId::new(),
        workspace_id: workspace.id.to_string(),
        title: title.to_string(),
        prompt: "haz algo".to_string(),
        branch: format!("pi/{title}"),
        base_branch: "main".to_string(),
        worktree_path: PathBuf::from(format!("/wt/{title}")),
        status: TaskStatus::Backlog,
        priority: TaskPriority::default(),
        error: None,
        pending_dialogs: Vec::new(),
        subagents: SubagentActivity::default(),
        created_at: now,
        updated_at: now,
    };
    TaskRepositoryAdapter::new(db.pool.clone())
        .insert(&task)
        .await
        .expect("task");
    task
}

fn image(name: &str, path: &str) -> StoredAttachment {
    StoredAttachment {
        id: format!("id-{name}-{path}"),
        name: name.to_string(),
        mime_type: "image/png".to_string(),
        path: path.to_string(),
        size_bytes: 1234,
    }
}

#[tokio::test(flavor = "multi_thread")]
async fn lists_images_in_insertion_order() {
    let db: common::TestDb = common::test_db().await;
    let task: Task = backlog_task(&db, "uno").await;
    let adapter: TaskAttachmentRepositoryAdapter =
        TaskAttachmentRepositoryAdapter::new(db.pool.clone());
    let images: Vec<StoredAttachment> = vec![
        image("c.png", "t/c.png"),
        image("a.png", "t/a.png"),
        image("b.png", "t/b.png"),
    ];

    adapter.insert_many(task.id, &images).await.expect("insert");
    let stored: Vec<StoredAttachment> = adapter.list_by_task(task.id).await.expect("list");

    assert_eq!(stored, images);
    assert_eq!(stored[0].path, "t/c.png");
    assert_eq!(stored[0].size_bytes, 1234);
}

#[tokio::test(flavor = "multi_thread")]
async fn lists_only_the_images_of_the_requested_task() {
    let db: common::TestDb = common::test_db().await;
    let first: Task = backlog_task(&db, "uno").await;
    let second: Task = backlog_task(&db, "dos").await;
    let adapter: TaskAttachmentRepositoryAdapter =
        TaskAttachmentRepositoryAdapter::new(db.pool.clone());
    adapter
        .insert_many(first.id, &[image("a.png", "t/a.png")])
        .await
        .expect("insert");
    adapter
        .insert_many(second.id, &[image("b.png", "t/b.png")])
        .await
        .expect("insert");

    let stored: Vec<StoredAttachment> = adapter.list_by_task(second.id).await.expect("list");

    assert_eq!(stored, vec![image("b.png", "t/b.png")]);
    assert!(
        adapter
            .list_by_task(TaskId::new())
            .await
            .expect("list")
            .is_empty()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn inserting_nothing_is_a_no_op() {
    let db: common::TestDb = common::test_db().await;
    let task: Task = backlog_task(&db, "uno").await;
    let adapter: TaskAttachmentRepositoryAdapter =
        TaskAttachmentRepositoryAdapter::new(db.pool.clone());

    adapter.insert_many(task.id, &[]).await.expect("insert");

    assert!(
        adapter
            .list_by_task(task.id)
            .await
            .expect("list")
            .is_empty()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn insert_many_is_atomic_when_the_task_does_not_exist() {
    let db: common::TestDb = common::test_db().await;
    let adapter: TaskAttachmentRepositoryAdapter =
        TaskAttachmentRepositoryAdapter::new(db.pool.clone());
    let missing: TaskId = TaskId::new();

    let result: Result<(), TaskError> = adapter
        .insert_many(
            missing,
            &[image("a.png", "t/a.png"), image("b.png", "t/b.png")],
        )
        .await;

    assert!(matches!(result, Err(TaskError::Repository(_))));
    assert!(
        adapter
            .list_by_task(missing)
            .await
            .expect("list")
            .is_empty()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn images_are_deleted_with_their_task() {
    let db: common::TestDb = common::test_db().await;
    let task: Task = backlog_task(&db, "uno").await;
    let adapter: TaskAttachmentRepositoryAdapter =
        TaskAttachmentRepositoryAdapter::new(db.pool.clone());
    adapter
        .insert_many(task.id, &[image("a.png", "t/a.png")])
        .await
        .expect("insert");

    let deleted: bool = TaskRepositoryAdapter::new(db.pool.clone())
        .delete_if_backlog(task.id)
        .await
        .expect("delete");

    assert!(deleted);
    assert!(
        adapter
            .list_by_task(task.id)
            .await
            .expect("list")
            .is_empty()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn images_are_deleted_with_their_workspace_tasks() {
    let db: common::TestDb = common::test_db().await;
    let mut task: Task = backlog_task(&db, "uno").await;
    let repository: TaskRepositoryAdapter = TaskRepositoryAdapter::new(db.pool.clone());
    repository
        .update_status(task.id, TaskStatus::Merged, None)
        .await
        .expect("status");
    task.status = TaskStatus::Merged;
    let adapter: TaskAttachmentRepositoryAdapter =
        TaskAttachmentRepositoryAdapter::new(db.pool.clone());
    adapter
        .insert_many(task.id, &[image("a.png", "t/a.png")])
        .await
        .expect("insert");

    let deleted: Option<Vec<TaskId>> = repository
        .delete_inactive_by_workspace(task.workspace_id.clone())
        .await
        .expect("delete");

    assert_eq!(deleted, Some(vec![task.id]));
    assert!(
        adapter
            .list_by_task(task.id)
            .await
            .expect("list")
            .is_empty()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn inserting_again_appends_after_the_existing_images() {
    let db: common::TestDb = common::test_db().await;
    let task: Task = backlog_task(&db, "append").await;
    let adapter: TaskAttachmentRepositoryAdapter =
        TaskAttachmentRepositoryAdapter::new(db.pool.clone());
    let first: Vec<StoredAttachment> = vec![image("a.png", "t/a.png"), image("b.png", "t/b.png")];
    let second: Vec<StoredAttachment> = vec![image("c.png", "t/c.png"), image("d.png", "t/d.png")];

    adapter.insert_many(task.id, &first).await.expect("first");
    adapter.insert_many(task.id, &second).await.expect("second");

    let expected: Vec<StoredAttachment> = first.into_iter().chain(second).collect();
    assert_eq!(adapter.list_by_task(task.id).await.expect("list"), expected);
}

#[tokio::test(flavor = "multi_thread")]
async fn appending_to_a_task_does_not_depend_on_other_tasks_positions() {
    let db: common::TestDb = common::test_db().await;
    let first: Task = backlog_task(&db, "uno").await;
    let second: Task = backlog_task(&db, "dos").await;
    let adapter: TaskAttachmentRepositoryAdapter =
        TaskAttachmentRepositoryAdapter::new(db.pool.clone());
    adapter
        .insert_many(
            first.id,
            &[image("a.png", "t/a.png"), image("b.png", "t/b.png")],
        )
        .await
        .expect("insert");

    adapter
        .insert_many(second.id, &[image("c.png", "t/c.png")])
        .await
        .expect("insert");

    assert_eq!(
        adapter.list_by_task(second.id).await.expect("list"),
        vec![image("c.png", "t/c.png")]
    );
}
