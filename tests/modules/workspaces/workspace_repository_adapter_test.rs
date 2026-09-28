#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use chrono::{DateTime, SubsecRound, Utc};
use pi_workflow::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use pi_workflow::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use pi_workflow::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use pi_workflow::modules::workspaces::infrastructure::adapters::outbound::persistence::workspace_repository_adapter::WorkspaceRepositoryAdapter;

fn workspace(name: &str, path: &str) -> Workspace {
    let created_at: DateTime<Utc> = Utc::now().trunc_subsecs(6);
    Workspace {
        id: WorkspaceId::new(),
        name: name.to_string(),
        path: PathBuf::from(path),
        base_branch: "main".to_string(),
        created_at,
    }
}

#[tokio::test(flavor = "multi_thread")]
async fn saves_and_finds_workspace() {
    let db: common::TestDb = common::test_db().await;
    let adapter: WorkspaceRepositoryAdapter = WorkspaceRepositoryAdapter::new(db.pool.clone());
    let original: Workspace = workspace("demo", "/repos/demo");

    adapter.save(&original).await.expect("save");

    let by_id: Option<Workspace> = adapter.find_by_id(original.id).await.expect("find");
    let by_path: Option<Workspace> = adapter
        .find_by_path(&PathBuf::from("/repos/demo"))
        .await
        .expect("find");
    assert_eq!(by_id, Some(original.clone()));
    assert_eq!(by_path, Some(original));
}

#[tokio::test(flavor = "multi_thread")]
async fn lists_all_workspaces() {
    let db: common::TestDb = common::test_db().await;
    let adapter: WorkspaceRepositoryAdapter = WorkspaceRepositoryAdapter::new(db.pool.clone());
    adapter
        .save(&workspace("a", "/repos/a"))
        .await
        .expect("save");
    adapter
        .save(&workspace("b", "/repos/b"))
        .await
        .expect("save");

    let all: Vec<Workspace> = adapter.list().await.expect("list");

    assert_eq!(all.len(), 2);
}

#[tokio::test(flavor = "multi_thread")]
async fn duplicate_path_is_already_registered() {
    let db: common::TestDb = common::test_db().await;
    let adapter: WorkspaceRepositoryAdapter = WorkspaceRepositoryAdapter::new(db.pool.clone());
    adapter
        .save(&workspace("a", "/repos/same"))
        .await
        .expect("save");

    let result: Result<(), WorkspaceError> = adapter.save(&workspace("b", "/repos/same")).await;

    assert_eq!(
        result,
        Err(WorkspaceError::AlreadyRegistered("/repos/same".to_string()))
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn delete_removes_and_reports_missing() {
    let db: common::TestDb = common::test_db().await;
    let adapter: WorkspaceRepositoryAdapter = WorkspaceRepositoryAdapter::new(db.pool.clone());
    let saved: Workspace = workspace("a", "/repos/a");
    adapter.save(&saved).await.expect("save");

    adapter.delete(saved.id).await.expect("delete");
    let again: Result<(), WorkspaceError> = adapter.delete(saved.id).await;

    assert_eq!(adapter.find_by_id(saved.id).await.expect("find"), None);
    assert_eq!(again, Err(WorkspaceError::NotFound(saved.id.to_string())));
}
