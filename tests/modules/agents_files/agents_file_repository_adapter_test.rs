#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use chrono::{DateTime, SubsecRound, Utc};
use pi_workflow::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use pi_workflow::modules::agents_files::domain::models::agents_file::{AgentsFile, AgentsFileId};
use pi_workflow::modules::agents_files::domain::ports::outbound::agents_file_repository_port::AgentsFileRepositoryPort;
use pi_workflow::modules::agents_files::infrastructure::adapters::outbound::persistence::agents_file_repository_adapter::AgentsFileRepositoryAdapter;

fn agents_file(description: &str, path: &str) -> AgentsFile {
    let created_at: DateTime<Utc> = Utc::now().trunc_subsecs(6);
    AgentsFile {
        id: AgentsFileId::new(),
        description: description.to_string(),
        path: PathBuf::from(path),
        created_at,
    }
}

#[tokio::test(flavor = "multi_thread")]
async fn saves_and_finds_by_id() {
    let db: common::TestDb = common::test_db().await;
    let adapter: AgentsFileRepositoryAdapter = AgentsFileRepositoryAdapter::new(db.pool.clone());
    let original: AgentsFile = agents_file("reglas", "/data/a.md");

    adapter.save(&original).await.expect("save");

    let found: Option<AgentsFile> = adapter.find_by_id(original.id).await.expect("find");
    let missing: Option<AgentsFile> = adapter.find_by_id(AgentsFileId::new()).await.expect("find");
    assert_eq!(found, Some(original));
    assert_eq!(missing, None);
}

#[tokio::test(flavor = "multi_thread")]
async fn lists_all_files() {
    let db: common::TestDb = common::test_db().await;
    let adapter: AgentsFileRepositoryAdapter = AgentsFileRepositoryAdapter::new(db.pool.clone());
    adapter
        .save(&agents_file("a", "/data/a.md"))
        .await
        .expect("save");
    adapter
        .save(&agents_file("b", "/data/b.md"))
        .await
        .expect("save");

    let all: Vec<AgentsFile> = adapter.list().await.expect("list");

    assert_eq!(all.len(), 2);
}

#[tokio::test(flavor = "multi_thread")]
async fn duplicate_path_is_rejected() {
    let db: common::TestDb = common::test_db().await;
    let adapter: AgentsFileRepositoryAdapter = AgentsFileRepositoryAdapter::new(db.pool.clone());
    adapter
        .save(&agents_file("a", "/data/same.md"))
        .await
        .expect("save");

    let result: Result<(), AgentsFileError> =
        adapter.save(&agents_file("b", "/data/same.md")).await;

    assert!(matches!(result, Err(AgentsFileError::Repository(_))));
}

#[tokio::test(flavor = "multi_thread")]
async fn delete_removes_and_reports_missing() {
    let db: common::TestDb = common::test_db().await;
    let adapter: AgentsFileRepositoryAdapter = AgentsFileRepositoryAdapter::new(db.pool.clone());
    let saved: AgentsFile = agents_file("a", "/data/a.md");
    adapter.save(&saved).await.expect("save");

    adapter.delete(saved.id).await.expect("delete");
    let again: Result<(), AgentsFileError> = adapter.delete(saved.id).await;

    assert_eq!(adapter.find_by_id(saved.id).await.expect("find"), None);
    assert_eq!(again, Err(AgentsFileError::NotFound(saved.id.to_string())));
}
