use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct TaskDiffDto {
    pub stat: String,
    pub patch: String,
    pub truncated: bool,
    pub commits: Vec<String>,
}

#[cfg(feature = "ssr")]
impl From<crate::modules::tasks::domain::models::worktree::TaskDiff> for TaskDiffDto {
    fn from(diff: crate::modules::tasks::domain::models::worktree::TaskDiff) -> Self {
        Self {
            stat: diff.stat,
            patch: diff.patch,
            truncated: diff.truncated,
            commits: diff.commits,
        }
    }
}
