use std::path::Path;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::directory_contents::DirectoryContents;

#[async_trait::async_trait]
pub trait DirectoryReaderPort: Send + Sync {
    /// Lists one directory level of `relative` (empty = root), confined to `root`.
    async fn list(
        &self,
        root: &Path,
        relative: &str,
    ) -> Result<DirectoryContents, ProjectTreeError>;
}
