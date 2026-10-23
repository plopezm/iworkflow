package models

// GitChangeKind says how a path differs from git HEAD.
type GitChangeKind uint8

const (
	GitChangeModified GitChangeKind = iota + 1
	GitChangeAdded
	GitChangeUntracked
	GitChangeDeleted
	GitChangeRenamed
	GitChangeConflicted
)

// GitChange is one changed path, relative to the project root and `/`-separated.
type GitChange struct {
	Path string
	Kind GitChangeKind
	// Staged means the index holds a change for this path (it will be part of the next commit).
	Staged bool
	// Unstaged means the working tree differs from the index (including untracked files).
	Unstaged bool
}

// WorkspaceRoot is the name and the root directory of a workspace or task worktree.
type WorkspaceRoot struct {
	Name string
	Path string
}
