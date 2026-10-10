use std::path::Path;

use iworkflow::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use iworkflow::modules::project_tree::domain::models::directory_contents::DirectoryContents;
use iworkflow::modules::project_tree::domain::models::tree_entry::TreeEntryKind;
use iworkflow::modules::project_tree::domain::ports::outbound::directory_reader_port::DirectoryReaderPort;
use iworkflow::modules::project_tree::infrastructure::adapters::outbound::fs::fs_directory_reader_adapter::FsDirectoryReaderAdapter;
use tempfile::TempDir;

async fn list(root: &Path, relative: &str) -> Result<DirectoryContents, ProjectTreeError> {
    let reader: Box<dyn DirectoryReaderPort> = Box::new(FsDirectoryReaderAdapter::new());
    reader.list(root, relative).await
}

#[tokio::test]
async fn lists_root_with_dirs_first_and_hides_git() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    std::fs::create_dir_all(dir.path().join(".git")).expect("git");
    std::fs::create_dir_all(dir.path().join("src")).expect("src");
    std::fs::write(dir.path().join("README.md"), "").expect("readme");
    std::fs::write(dir.path().join(".gitignore"), "").expect("ignore");

    let contents: DirectoryContents = list(dir.path(), "").await.expect("list");

    let names: Vec<&str> = contents.entries.iter().map(|e| e.name.as_str()).collect();
    assert_eq!(names, vec!["src", ".gitignore", "README.md"]);
    assert_eq!(contents.entries[0].kind, TreeEntryKind::Directory);
}

#[tokio::test]
async fn lists_nested_directory_with_relative_paths() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    std::fs::create_dir_all(dir.path().join("a/b")).expect("dirs");
    std::fs::write(dir.path().join("a/b/c.txt"), "").expect("file");

    let contents: DirectoryContents = list(dir.path(), "a/b").await.expect("list");

    assert_eq!(contents.path, "a/b");
    assert_eq!(contents.entries.len(), 1);
    assert_eq!(contents.entries[0].path, "a/b/c.txt");
}

#[tokio::test]
async fn rejects_traversal_and_absolute_paths() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    assert_eq!(
        list(dir.path(), "..").await,
        Err(ProjectTreeError::InvalidPath("..".to_string()))
    );
    assert_eq!(
        list(dir.path(), "/tmp").await,
        Err(ProjectTreeError::InvalidPath("/tmp".to_string()))
    );
}

#[cfg(unix)]
#[tokio::test]
async fn rejects_symlink_escaping_the_root() {
    let root: TempDir = TempDir::new().expect("root");
    let outside: TempDir = TempDir::new().expect("outside");
    std::fs::write(outside.path().join("secret.txt"), "").expect("secret");
    std::os::unix::fs::symlink(outside.path(), root.path().join("escape")).expect("symlink");

    assert_eq!(
        list(root.path(), "escape").await,
        Err(ProjectTreeError::PathOutsideRoot("escape".to_string()))
    );
    let contents: DirectoryContents = list(root.path(), "").await.expect("list");
    assert!(contents.entries.is_empty());
}

#[tokio::test]
async fn reports_missing_and_non_directory_paths() {
    let dir: TempDir = TempDir::new().expect("tempdir");
    std::fs::write(dir.path().join("f.txt"), "").expect("file");
    assert_eq!(
        list(dir.path(), "missing").await,
        Err(ProjectTreeError::PathNotFound("missing".to_string()))
    );
    assert_eq!(
        list(dir.path(), "f.txt").await,
        Err(ProjectTreeError::NotADirectory("f.txt".to_string()))
    );
}
