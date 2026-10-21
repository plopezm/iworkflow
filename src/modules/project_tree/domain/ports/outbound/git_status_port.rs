use std::path::Path;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::git_change::GitChange;

#[async_trait::async_trait]
pub trait GitStatusPort: Send + Sync {
    /// Paths under `root` that differ from `HEAD`. Empty when `root` is not a git repository.
    async fn changes(&self, root: &Path) -> Result<Vec<GitChange>, ProjectTreeError>;

    /// Content of `relative` at `HEAD`; `None` when git does not know the file (new or not a repo).
    async fn base_content(
        &self,
        root: &Path,
        relative: &str,
    ) -> Result<Option<String>, ProjectTreeError>;

    /// Adds `paths` (relative to `root`; empty means everything) to the index.
    async fn stage(&self, root: &Path, paths: &[String]) -> Result<(), ProjectTreeError>;

    /// Removes `paths` (relative to `root`; empty means everything) from the index, keeping the
    /// working tree untouched.
    async fn unstage(&self, root: &Path, paths: &[String]) -> Result<(), ProjectTreeError>;
}
