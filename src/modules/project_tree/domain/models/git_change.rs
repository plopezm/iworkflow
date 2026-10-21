/// How a path differs from git `HEAD`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum GitChangeKind {
    Modified,
    Added,
    Untracked,
    Deleted,
    Renamed,
    Conflicted,
}

/// One changed path, relative to the project root and `/`-separated.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GitChange {
    pub path: String,
    pub kind: GitChangeKind,
    /// The index holds a change for this path (it will be part of the next commit).
    pub staged: bool,
    /// The working tree differs from the index (including untracked files).
    pub unstaged: bool,
}
