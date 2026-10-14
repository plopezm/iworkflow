use std::path::Path;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task_file_content::TaskFileContent;

/// Reads and overwrites single text files inside a task worktree.
#[async_trait::async_trait]
pub trait TaskFilePort: Send + Sync {
    /// Reads `rel` (relative to `root`). Rejects paths escaping `root`, `.git`, oversized and
    /// non-UTF-8 files.
    async fn read(&self, root: &Path, rel: &str) -> Result<TaskFileContent, TaskError>;

    /// Overwrites the existing file `rel`; never creates files or directories.
    async fn write(&self, root: &Path, rel: &str, content: &str) -> Result<(), TaskError>;
}
