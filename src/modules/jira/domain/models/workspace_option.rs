/// A workspace the issue can be converted into a task of.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkspaceOption {
    pub id: String,
    pub name: String,
    pub base_branch: String,
}

/// A branch a task worktree can start from.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BranchOption {
    pub name: String,
    pub remote: bool,
}
