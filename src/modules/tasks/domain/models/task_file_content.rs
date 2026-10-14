/// Text file read from a task worktree; `path` is relative to the worktree root.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TaskFileContent {
    pub path: String,
    pub content: String,
}
