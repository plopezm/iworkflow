use std::cmp::Ordering;
use std::path::{Component, Path, PathBuf};

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::directory_contents::DirectoryContents;
use crate::modules::project_tree::domain::models::tree_entry::{TreeEntry, TreeEntryKind};
use crate::modules::project_tree::domain::ports::outbound::directory_reader_port::DirectoryReaderPort;

const HIDDEN_ENTRY: &str = ".git";

/// Reads one directory level from the local filesystem, confined to the project root.
#[derive(Default)]
pub struct FsDirectoryReaderAdapter;

impl FsDirectoryReaderAdapter {
    pub fn new() -> Self {
        Self
    }
}

pub(super) fn io_error(error: std::io::Error) -> ProjectTreeError {
    ProjectTreeError::Io(error.to_string())
}

/// Accepts only plain `/`-separated names: no absolute paths, `.`, `..` or prefixes.
pub fn validate_relative(relative: &str) -> Result<(), ProjectTreeError> {
    if relative.is_empty() {
        return Ok(());
    }
    for segment in relative.split('/') {
        let components: Vec<Component> = Path::new(segment).components().collect();
        if !matches!(components.as_slice(), [Component::Normal(_)]) {
            return Err(ProjectTreeError::InvalidPath(relative.to_string()));
        }
    }
    Ok(())
}

pub(super) async fn canonicalize(path: &Path, label: &str) -> Result<PathBuf, ProjectTreeError> {
    tokio::fs::canonicalize(path)
        .await
        .map_err(|error: std::io::Error| match error.kind() {
            std::io::ErrorKind::NotFound => ProjectTreeError::PathNotFound(label.to_string()),
            _ => io_error(error),
        })
}

fn join_relative(relative: &str, name: &str) -> String {
    if relative.is_empty() {
        name.to_string()
    } else {
        format!("{relative}/{name}")
    }
}

fn rank(entry: &TreeEntry) -> u8 {
    match entry.kind {
        TreeEntryKind::Directory => 0,
        TreeEntryKind::File => 1,
    }
}

fn compare(a: &TreeEntry, b: &TreeEntry) -> Ordering {
    rank(a)
        .cmp(&rank(b))
        .then_with(|| a.name.to_lowercase().cmp(&b.name.to_lowercase()))
        .then_with(|| a.name.cmp(&b.name))
}

#[async_trait::async_trait]
impl DirectoryReaderPort for FsDirectoryReaderAdapter {
    async fn list(
        &self,
        root: &Path,
        relative: &str,
    ) -> Result<DirectoryContents, ProjectTreeError> {
        validate_relative(relative)?;
        let canonical_root: PathBuf = canonicalize(root, "").await?;
        let target: PathBuf = canonicalize(&root.join(relative), relative).await?;
        if !target.starts_with(&canonical_root) {
            return Err(ProjectTreeError::PathOutsideRoot(relative.to_string()));
        }
        let metadata: std::fs::Metadata = tokio::fs::metadata(&target).await.map_err(io_error)?;
        if !metadata.is_dir() {
            return Err(ProjectTreeError::NotADirectory(relative.to_string()));
        }

        let mut reader: tokio::fs::ReadDir =
            tokio::fs::read_dir(&target).await.map_err(io_error)?;
        let mut entries: Vec<TreeEntry> = Vec::new();
        while let Some(item) = reader.next_entry().await.map_err(io_error)? {
            let name: String = item.file_name().to_string_lossy().into_owned();
            if name == HIDDEN_ENTRY {
                continue;
            }
            let entry_path: PathBuf = item.path();
            // Following symlinks: broken links and links escaping the root are skipped.
            let Ok(resolved) = tokio::fs::canonicalize(&entry_path).await else {
                continue;
            };
            if !resolved.starts_with(&canonical_root) {
                continue;
            }
            let Ok(entry_metadata) = tokio::fs::metadata(&entry_path).await else {
                continue;
            };
            let kind: TreeEntryKind = if entry_metadata.is_dir() {
                TreeEntryKind::Directory
            } else {
                TreeEntryKind::File
            };
            entries.push(TreeEntry {
                path: join_relative(relative, &name),
                name,
                kind,
            });
        }
        entries.sort_by(compare);
        Ok(DirectoryContents {
            path: relative.to_string(),
            entries,
        })
    }
}

#[cfg(test)]
mod tests {
    use tempfile::TempDir;

    use super::*;

    async fn list(root: &Path, relative: &str) -> Result<DirectoryContents, ProjectTreeError> {
        FsDirectoryReaderAdapter::new().list(root, relative).await
    }

    fn names(contents: &DirectoryContents) -> Vec<&str> {
        contents
            .entries
            .iter()
            .map(|entry: &TreeEntry| entry.name.as_str())
            .collect()
    }

    #[tokio::test]
    async fn sorts_directories_first_then_files_case_insensitive() {
        let dir: TempDir = TempDir::new().unwrap();
        std::fs::create_dir(dir.path().join("beta")).unwrap();
        std::fs::create_dir(dir.path().join("Alpha")).unwrap();
        std::fs::write(dir.path().join("zeta.txt"), "").unwrap();
        std::fs::write(dir.path().join("Apple.txt"), "").unwrap();

        let contents: DirectoryContents = list(dir.path(), "").await.unwrap();

        assert_eq!(
            names(&contents),
            vec!["Alpha", "beta", "Apple.txt", "zeta.txt"]
        );
        assert_eq!(contents.path, "");
        assert_eq!(contents.entries[0].kind, TreeEntryKind::Directory);
        assert_eq!(contents.entries[2].kind, TreeEntryKind::File);
    }

    #[tokio::test]
    async fn hides_git_but_shows_other_dotfiles() {
        let dir: TempDir = TempDir::new().unwrap();
        std::fs::create_dir(dir.path().join(".git")).unwrap();
        std::fs::write(dir.path().join(".gitignore"), "").unwrap();
        std::fs::write(dir.path().join(".env"), "").unwrap();

        let contents: DirectoryContents = list(dir.path(), "").await.unwrap();

        assert_eq!(names(&contents), vec![".env", ".gitignore"]);
    }

    #[tokio::test]
    async fn lists_nested_relative_path() {
        let dir: TempDir = TempDir::new().unwrap();
        std::fs::create_dir_all(dir.path().join("src/ui")).unwrap();
        std::fs::write(dir.path().join("src/main.rs"), "").unwrap();

        let contents: DirectoryContents = list(dir.path(), "src").await.unwrap();

        assert_eq!(contents.path, "src");
        assert_eq!(names(&contents), vec!["ui", "main.rs"]);
        assert_eq!(contents.entries[0].path, "src/ui");
        assert_eq!(contents.entries[1].path, "src/main.rs");
    }

    #[tokio::test]
    async fn rejects_parent_traversal() {
        let dir: TempDir = TempDir::new().unwrap();
        let result: Result<DirectoryContents, ProjectTreeError> = list(dir.path(), "../x").await;
        assert_eq!(
            result,
            Err(ProjectTreeError::InvalidPath("../x".to_string()))
        );
    }

    #[tokio::test]
    async fn rejects_absolute_path() {
        let dir: TempDir = TempDir::new().unwrap();
        let result: Result<DirectoryContents, ProjectTreeError> = list(dir.path(), "/etc").await;
        assert_eq!(
            result,
            Err(ProjectTreeError::InvalidPath("/etc".to_string()))
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn rejects_symlink_pointing_outside_root() {
        let root: TempDir = TempDir::new().unwrap();
        let outside: TempDir = TempDir::new().unwrap();
        std::os::unix::fs::symlink(outside.path(), root.path().join("link")).unwrap();

        let result: Result<DirectoryContents, ProjectTreeError> = list(root.path(), "link").await;
        assert_eq!(
            result,
            Err(ProjectTreeError::PathOutsideRoot("link".to_string()))
        );

        let contents: DirectoryContents = list(root.path(), "").await.unwrap();
        assert!(contents.entries.is_empty());
    }

    #[tokio::test]
    async fn file_path_is_not_a_directory() {
        let dir: TempDir = TempDir::new().unwrap();
        std::fs::write(dir.path().join("a.txt"), "").unwrap();
        let result: Result<DirectoryContents, ProjectTreeError> = list(dir.path(), "a.txt").await;
        assert_eq!(
            result,
            Err(ProjectTreeError::NotADirectory("a.txt".to_string()))
        );
    }

    #[tokio::test]
    async fn missing_path_is_not_found() {
        let dir: TempDir = TempDir::new().unwrap();
        let result: Result<DirectoryContents, ProjectTreeError> = list(dir.path(), "nope").await;
        assert_eq!(
            result,
            Err(ProjectTreeError::PathNotFound("nope".to_string()))
        );
    }
}
