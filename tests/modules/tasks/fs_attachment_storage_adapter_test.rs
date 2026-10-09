use std::path::Path;

use base64::Engine as _;
use base64::engine::general_purpose::STANDARD;
use iworkflow::modules::tasks::domain::errors::task_error::TaskError;
use iworkflow::modules::tasks::domain::models::image_attachment::ImageAttachment;
use iworkflow::modules::tasks::domain::models::task::TaskId;
use iworkflow::modules::tasks::domain::ports::outbound::attachment_storage_port::AttachmentStoragePort;
use iworkflow::modules::tasks::infrastructure::adapters::outbound::storage::fs_attachment_storage_adapter::FsAttachmentStorageAdapter;
use tempfile::TempDir;

const BYTES: &[u8] = b"\x89PNG fake image bytes";

fn image(mime: &str) -> ImageAttachment {
    ImageAttachment {
        name: "foto".to_string(),
        mime_type: mime.to_string(),
        data: STANDARD.encode(BYTES),
    }
}

fn adapter(dir: &TempDir) -> FsAttachmentStorageAdapter {
    FsAttachmentStorageAdapter::new(dir.path().join("attachments"))
}

fn entries(dir: &Path) -> Vec<String> {
    match std::fs::read_dir(dir) {
        Ok(read) => read
            .map(|e| e.expect("entry").file_name().to_string_lossy().into_owned())
            .collect(),
        Err(_) => Vec::new(),
    }
}

#[tokio::test]
async fn saves_and_reads_back_the_same_bytes() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAttachmentStorageAdapter = adapter(&dir);
    let task_id: TaskId = TaskId::new();

    let path: String = storage
        .save(task_id, "att-1", &image("image/png"))
        .await
        .expect("save");

    assert_eq!(path, format!("{task_id}/att-1.png"));
    let on_disk: Vec<u8> = std::fs::read(dir.path().join("attachments").join(&path)).expect("file");
    assert_eq!(on_disk, BYTES);
    assert_eq!(
        storage.read(&path).await.expect("read"),
        STANDARD.encode(BYTES)
    );
}

#[tokio::test]
async fn maps_mime_types_to_extensions() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAttachmentStorageAdapter = adapter(&dir);
    let task_id: TaskId = TaskId::new();
    let expected: [(&str, &str); 4] = [
        ("image/png", "png"),
        ("image/jpeg", "jpg"),
        ("image/gif", "gif"),
        ("image/webp", "webp"),
    ];

    for (mime, ext) in expected {
        let path: String = storage
            .save(task_id, "att", &image(mime))
            .await
            .expect("save");
        assert_eq!(path, format!("{task_id}/att.{ext}"));
    }
}

#[tokio::test]
async fn rejects_unsupported_mime_and_invalid_base64() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAttachmentStorageAdapter = adapter(&dir);
    let task_id: TaskId = TaskId::new();
    let bad_data: ImageAttachment = ImageAttachment {
        data: "%%%no-base64".to_string(),
        ..image("image/png")
    };

    assert!(matches!(
        storage.save(task_id, "a", &image("image/bmp")).await,
        Err(TaskError::Storage(_))
    ));
    assert!(matches!(
        storage.save(task_id, "a", &bad_data).await,
        Err(TaskError::Storage(_))
    ));
    assert!(entries(&dir.path().join("attachments").join(task_id.to_string())).is_empty());
}

#[tokio::test]
async fn write_is_atomic_and_leaves_no_temporary_files() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAttachmentStorageAdapter = adapter(&dir);
    let task_id: TaskId = TaskId::new();

    storage
        .save(task_id, "att-1", &image("image/png"))
        .await
        .expect("first");
    storage
        .save(task_id, "att-1", &image("image/png"))
        .await
        .expect("overwrite");

    let task_dir = dir.path().join("attachments").join(task_id.to_string());
    assert_eq!(entries(&task_dir), vec!["att-1.png".to_string()]);
}

#[tokio::test]
async fn rejects_path_traversal_in_ids_and_paths() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAttachmentStorageAdapter = adapter(&dir);
    let outside = dir.path().join("secret.txt");
    std::fs::write(&outside, b"secret").expect("secret");

    for path in [
        "../secret.txt",
        "/etc/passwd",
        "a/../../secret.txt",
        "",
        ".",
    ] {
        assert!(
            matches!(storage.read(path).await, Err(TaskError::Storage(_))),
            "read {path}"
        );
        assert!(
            matches!(storage.delete(path).await, Err(TaskError::Storage(_))),
            "delete {path}"
        );
    }
    assert!(matches!(
        storage
            .save(TaskId::new(), "../evil", &image("image/png"))
            .await,
        Err(TaskError::Storage(_))
    ));
    assert!(outside.exists());
}

#[tokio::test]
async fn reading_a_missing_file_fails_with_a_clear_error() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAttachmentStorageAdapter = adapter(&dir);

    let result: Result<String, TaskError> = storage.read("nada/ninguno.png").await;

    assert!(
        matches!(result, Err(TaskError::Storage(message)) if message.contains("nada/ninguno.png"))
    );
}

#[tokio::test]
async fn delete_removes_one_file_and_is_idempotent() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAttachmentStorageAdapter = adapter(&dir);
    let task_id: TaskId = TaskId::new();
    let first: String = storage
        .save(task_id, "a", &image("image/png"))
        .await
        .expect("a");
    let second: String = storage
        .save(task_id, "b", &image("image/png"))
        .await
        .expect("b");

    storage.delete(&first).await.expect("delete");
    storage.delete(&first).await.expect("again");

    assert!(storage.read(&first).await.is_err());
    assert!(storage.read(&second).await.is_ok());
}

#[tokio::test]
async fn delete_task_dir_removes_only_that_task_and_is_idempotent() {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let storage: FsAttachmentStorageAdapter = adapter(&dir);
    let task_id: TaskId = TaskId::new();
    let other: TaskId = TaskId::new();
    let path: String = storage
        .save(task_id, "a", &image("image/png"))
        .await
        .expect("a");
    let other_path: String = storage
        .save(other, "a", &image("image/png"))
        .await
        .expect("other");

    storage.delete_task_dir(task_id).await.expect("delete");
    storage.delete_task_dir(task_id).await.expect("again");
    storage
        .delete_task_dir(TaskId::new())
        .await
        .expect("never existed");

    assert!(storage.read(&path).await.is_err());
    assert!(storage.read(&other_path).await.is_ok());
}
