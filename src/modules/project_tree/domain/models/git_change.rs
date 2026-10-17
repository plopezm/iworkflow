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
}
