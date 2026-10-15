use std::path::Path;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::project_file::ProjectFile;

#[async_trait::async_trait]
pub trait FileStorePort: Send + Sync {
    /// Reads the UTF-8 text file at `relative`, confined to `root`.
    async fn read(&self, root: &Path, relative: &str) -> Result<ProjectFile, ProjectTreeError>;

    /// Overwrites the existing text file at `relative`, confined to `root`.
    async fn write(
        &self,
        root: &Path,
        relative: &str,
        content: &str,
    ) -> Result<(), ProjectTreeError>;
}
