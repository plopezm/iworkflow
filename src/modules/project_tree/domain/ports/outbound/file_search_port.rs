use std::path::Path;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;

#[async_trait::async_trait]
pub trait FileSearchPort: Send + Sync {
    /// Every file under `root` that is not git-ignored, as `/`-separated relative paths.
    /// Empty when `root` is not a git repository.
    async fn files(&self, root: &Path) -> Result<Vec<String>, ProjectTreeError>;
}
