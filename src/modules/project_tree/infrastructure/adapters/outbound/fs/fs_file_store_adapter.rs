use std::path::{Path, PathBuf};

use super::fs_directory_reader_adapter::{canonicalize, io_error, validate_relative};
use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::project_file::ProjectFile;
use crate::modules::project_tree::domain::ports::outbound::file_store_port::FileStorePort;

/// Largest file the viewer/editor loads, in bytes.
const MAX_FILE_BYTES: u64 = 2 * 1024 * 1024;

/// Reads and overwrites text files of the local filesystem, confined to the project root.
#[derive(Default)]
pub struct FsFileStoreAdapter;

impl FsFileStoreAdapter {
    pub fn new() -> Self {
        Self
    }
}

/// Resolves `relative` to an existing regular file inside `root`.
async fn resolve_file(root: &Path, relative: &str) -> Result<PathBuf, ProjectTreeError> {
    if relative.is_empty() {
        return Err(ProjectTreeError::NotAFile(String::new()));
    }
    validate_relative(relative)?;
    let canonical_root: PathBuf = canonicalize(root, "").await?;
    let target: PathBuf = canonicalize(&root.join(relative), relative).await?;
    if !target.starts_with(&canonical_root) {
        return Err(ProjectTreeError::PathOutsideRoot(relative.to_string()));
    }
    let metadata: std::fs::Metadata = tokio::fs::metadata(&target).await.map_err(io_error)?;
    if !metadata.is_file() {
        return Err(ProjectTreeError::NotAFile(relative.to_string()));
    }
    if metadata.len() > MAX_FILE_BYTES {
        return Err(ProjectTreeError::FileTooLarge(relative.to_string()));
    }
    Ok(target)
}

#[async_trait::async_trait]
impl FileStorePort for FsFileStoreAdapter {
    async fn read(&self, root: &Path, relative: &str) -> Result<ProjectFile, ProjectTreeError> {
        let target: PathBuf = resolve_file(root, relative).await?;
        let bytes: Vec<u8> = tokio::fs::read(&target).await.map_err(io_error)?;
        let content: String = String::from_utf8(bytes)
            .map_err(|_| ProjectTreeError::NotText(relative.to_string()))?;
        let name: String = relative.rsplit('/').next().unwrap_or(relative).to_string();
        Ok(ProjectFile {
            name,
            path: relative.to_string(),
            content,
        })
    }

    async fn write(
        &self,
        root: &Path,
        relative: &str,
        content: &str,
    ) -> Result<(), ProjectTreeError> {
        if content.len() as u64 > MAX_FILE_BYTES {
            return Err(ProjectTreeError::FileTooLarge(relative.to_string()));
        }
        let target: PathBuf = resolve_file(root, relative).await?;
        tokio::fs::write(&target, content).await.map_err(io_error)
    }
}
