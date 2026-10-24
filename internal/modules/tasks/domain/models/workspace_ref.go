package models

// WorkspaceRef is the read-only view of a workspace the tasks module needs. It is provided by the
// workspaces module through WorkspaceLookup.
type WorkspaceRef struct {
	ID         string
	Name       string
	Path       string
	BaseBranch string
}

// BranchOption is a branch offered when a task chooses its base.
type BranchOption struct {
	Name    string
	Remote  bool
	Current bool
}
