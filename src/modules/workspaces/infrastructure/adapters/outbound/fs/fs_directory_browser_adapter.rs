use std::path::{Path, PathBuf};

use tokio::fs::{self, DirEntry, ReadDir};

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::directory_listing::{
    DirectoryEntry, DirectoryListing,
};
use crate::modules::workspaces::domain::ports::outbound::directory_browser_port::DirectoryBrowserPort;

const DOT_PREFIX: char = '.';
const GIT_DIR_NAME: &str = ".git";

/// Lista carpetas del sistema de ficheros local usando `tokio::fs`.
#[derive(Default)]
pub struct FsDirectoryBrowserAdapter;

impl FsDirectoryBrowserAdapter {
    pub fn new() -> Self {
        Self
    }
}

fn io_error(path: &Path, error: std::io::Error) -> WorkspaceError {
    WorkspaceError::Io(format!("{}: {error}", path.display()))
}

/// Sube por los ancestros de `path` hasta encontrar una carpeta existente.
fn nearest_existing_ancestor(path: &Path) -> Option<PathBuf> {
    let mut current: &Path = path;
    loop {
        if current.is_dir() {
            return Some(current.to_path_buf());
        }
        current = current.parent()?;
    }
}

async fn is_git_repo(path: &Path) -> bool {
    fs::metadata(path.join(GIT_DIR_NAME)).await.is_ok()
}

async fn list_entries(dir: &Path) -> Result<Vec<DirectoryEntry>, WorkspaceError> {
    let mut read_dir: ReadDir = fs::read_dir(dir).await.map_err(|e| io_error(dir, e))?;
    let mut entries: Vec<DirectoryEntry> = Vec::new();
    while let Some(entry) = read_dir.next_entry().await.map_err(|e| io_error(dir, e))? {
        let entry: DirEntry = entry;
        let name: String = entry.file_name().to_string_lossy().into_owned();
        if name.starts_with(DOT_PREFIX) {
            continue;
        }
        let path: PathBuf = entry.path();
        let Ok(file_type) = fs::metadata(&path).await.map(|m| m.file_type()) else {
            continue;
        };
        if !file_type.is_dir() {
            continue;
        }
        let is_git_repo: bool = is_git_repo(&path).await;
        entries.push(DirectoryEntry {
            name,
            path,
            is_git_repo,
        });
    }
    entries.sort_by(|a: &DirectoryEntry, b: &DirectoryEntry| {
        a.name.to_lowercase().cmp(&b.name.to_lowercase())
    });
    Ok(entries)
}

#[async_trait::async_trait]
impl DirectoryBrowserPort for FsDirectoryBrowserAdapter {
    async fn list(&self, path: &Path) -> Result<DirectoryListing, WorkspaceError> {
        let target: PathBuf = nearest_existing_ancestor(path)
            .ok_or_else(|| WorkspaceError::PathNotFound(path.display().to_string()))?;
        let entries: Vec<DirectoryEntry> = list_entries(&target).await?;
        Ok(DirectoryListing {
            parent: target.parent().map(Path::to_path_buf),
            path: target,
            entries,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn lists_only_directories_hides_dot_entries_and_sorts_case_insensitively() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        std::fs::create_dir(dir.path().join("Beta")).expect("mkdir");
        std::fs::create_dir(dir.path().join("alpha")).expect("mkdir");
        std::fs::create_dir(dir.path().join(".hidden")).expect("mkdir");
        std::fs::write(dir.path().join("file.txt"), "x").expect("write");

        let listing: DirectoryListing = FsDirectoryBrowserAdapter::new()
            .list(dir.path())
            .await
            .expect("listado");

        let names: Vec<String> = listing.entries.iter().map(|e| e.name.clone()).collect();
        assert_eq!(names, vec!["alpha".to_string(), "Beta".to_string()]);
    }

    #[tokio::test]
    async fn marks_git_repositories() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        std::fs::create_dir(dir.path().join("repo")).expect("mkdir");
        std::fs::create_dir(dir.path().join("repo/.git")).expect("mkdir");
        std::fs::create_dir(dir.path().join("plain")).expect("mkdir");

        let listing: DirectoryListing = FsDirectoryBrowserAdapter::new()
            .list(dir.path())
            .await
            .expect("listado");

        let repo: &DirectoryEntry = listing
            .entries
            .iter()
            .find(|e| e.name == "repo")
            .expect("repo");
        let plain: &DirectoryEntry = listing
            .entries
            .iter()
            .find(|e| e.name == "plain")
            .expect("plain");
        assert!(repo.is_git_repo);
        assert!(!plain.is_git_repo);
    }

    #[tokio::test]
    async fn falls_back_to_nearest_existing_ancestor() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let missing: PathBuf = dir.path().join("missing/nested/child");

        let listing: DirectoryListing = FsDirectoryBrowserAdapter::new()
            .list(&missing)
            .await
            .expect("listado");

        assert_eq!(listing.path, dir.path());
    }

    #[tokio::test]
    async fn reports_parent_directory() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        std::fs::create_dir(dir.path().join("child")).expect("mkdir");

        let listing: DirectoryListing = FsDirectoryBrowserAdapter::new()
            .list(&dir.path().join("child"))
            .await
            .expect("listado");

        assert_eq!(listing.parent.as_deref(), Some(dir.path()));
    }
}
