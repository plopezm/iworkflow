use std::path::{Path, PathBuf};

use pi_workflow::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use pi_workflow::modules::agents_files::domain::models::agents_file::AgentsFileId;
use pi_workflow::modules::agents_files::domain::ports::outbound::agents_file_storage_port::AgentsFileStoragePort;
use pi_workflow::modules::agents_files::infrastructure::adapters::outbound::fs::fs_agents_file_storage_adapter::FsAgentsFileStorageAdapter;
use tempfile::TempDir;

fn adapter(dir: &TempDir) -> FsAgentsFileStorageAdapter {
    FsAgentsFileStorageAdapter::new(dir.path().join("agents-files"))
}

#[tokio::test]
async fn stores_reads_and_removes_a_file() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAgentsFileStorageAdapter = adapter(&dir);
    let id: AgentsFileId = AgentsFileId::new();

    let path: PathBuf = storage.store(&id, b"# reglas\n").await.expect("store");

    assert!(path.is_absolute());
    assert_eq!(
        path.file_name().and_then(|n| n.to_str()),
        Some(format!("{id}.md").as_str())
    );
    assert_eq!(
        storage.read(&path).await.expect("read"),
        b"# reglas\n".to_vec()
    );

    storage.remove(&path).await.expect("remove");
    assert!(!path.exists());
    storage.remove(&path).await.expect("remove missing is ok");
}

#[tokio::test]
async fn store_leaves_no_temporary_files() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAgentsFileStorageAdapter = adapter(&dir);

    storage
        .store(&AgentsFileId::new(), b"x")
        .await
        .expect("store");

    let entries: Vec<String> = std::fs::read_dir(dir.path().join("agents-files"))
        .expect("read_dir")
        .map(|e| e.expect("entry").file_name().to_string_lossy().into_owned())
        .collect();
    assert_eq!(entries.len(), 1);
    assert!(entries[0].ends_with(".md") && !entries[0].starts_with('.'));
}

#[tokio::test]
async fn rejects_paths_outside_the_base_dir() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAgentsFileStorageAdapter = adapter(&dir);
    storage
        .store(&AgentsFileId::new(), b"x")
        .await
        .expect("store");
    let foreign: PathBuf = dir.path().join("foreign.md");
    std::fs::write(&foreign, "secreto").expect("write");
    let traversal: PathBuf = dir
        .path()
        .join("agents-files")
        .join("..")
        .join("foreign.md");

    for path in [
        foreign.as_path(),
        traversal.as_path(),
        Path::new("/etc/hostname"),
    ] {
        let read: Result<Vec<u8>, AgentsFileError> = storage.read(path).await;
        let remove: Result<(), AgentsFileError> = storage.remove(path).await;
        assert!(matches!(read, Err(AgentsFileError::Storage(_))), "{path:?}");
        assert!(
            matches!(remove, Err(AgentsFileError::Storage(_))),
            "{path:?}"
        );
    }
    assert!(foreign.exists());
}
