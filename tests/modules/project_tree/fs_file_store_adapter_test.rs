use iworkflow::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use iworkflow::modules::project_tree::domain::models::project_file::ProjectFile;
use iworkflow::modules::project_tree::domain::ports::outbound::file_store_port::FileStorePort;
use iworkflow::modules::project_tree::infrastructure::adapters::outbound::fs::fs_file_store_adapter::FsFileStoreAdapter;
use tempfile::TempDir;

fn store() -> Box<dyn FileStorePort> {
    Box::new(FsFileStoreAdapter::new())
}

#[tokio::test]
async fn reads_nested_file_with_name_and_content() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    std::fs::create_dir_all(dir.path().join("src")).expect("dir");
    std::fs::write(dir.path().join("src/main.rs"), "fn main() {}").expect("file");

    let file: ProjectFile = store().read(dir.path(), "src/main.rs").await.expect("read");

    assert_eq!(file.name, "main.rs");
    assert_eq!(file.path, "src/main.rs");
    assert_eq!(file.content, "fn main() {}");
}

#[tokio::test]
async fn rejects_traversal_and_directories() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    std::fs::create_dir_all(dir.path().join("src")).expect("dir");

    assert_eq!(
        store().read(dir.path(), "../x").await,
        Err(ProjectTreeError::InvalidPath("../x".to_string()))
    );
    assert_eq!(
        store().read(dir.path(), "src").await,
        Err(ProjectTreeError::NotAFile("src".to_string()))
    );
    assert_eq!(
        store().read(dir.path(), "nope.txt").await,
        Err(ProjectTreeError::PathNotFound("nope.txt".to_string()))
    );
}

#[tokio::test]
async fn rejects_binary_content() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    std::fs::write(dir.path().join("a.bin"), [0xff, 0xfe, 0x00]).expect("file");

    assert_eq!(
        store().read(dir.path(), "a.bin").await,
        Err(ProjectTreeError::NotText("a.bin".to_string()))
    );
}

#[tokio::test]
async fn write_overwrites_existing_file_only() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    std::fs::write(dir.path().join("a.txt"), "old").expect("file");

    store()
        .write(dir.path(), "a.txt", "new")
        .await
        .expect("write");

    assert_eq!(
        std::fs::read_to_string(dir.path().join("a.txt")).unwrap(),
        "new"
    );
    assert_eq!(
        store().write(dir.path(), "missing.txt", "x").await,
        Err(ProjectTreeError::PathNotFound("missing.txt".to_string()))
    );
    assert!(!dir.path().join("missing.txt").exists());
}

#[cfg(unix)]
#[tokio::test]
async fn symlink_escaping_root_is_refused() {
    let outside: TempDir = TempDir::new().expect("outside");
    std::fs::write(outside.path().join("secret.txt"), "s").expect("secret");
    let dir: TempDir = TempDir::new().expect("tempdir");
    std::os::unix::fs::symlink(outside.path().join("secret.txt"), dir.path().join("link"))
        .expect("link");

    assert_eq!(
        store().read(dir.path(), "link").await,
        Err(ProjectTreeError::PathOutsideRoot("link".to_string()))
    );
    assert_eq!(
        store().write(dir.path(), "link", "x").await,
        Err(ProjectTreeError::PathOutsideRoot("link".to_string()))
    );
}
