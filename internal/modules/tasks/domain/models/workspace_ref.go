package models

// WorkspaceRef is the read-only view of a workspace the tasks module needs. It is provided by the
// workspaces module through WorkspaceLookup.
type WorkspaceRef struct {
	ID         string
	Name       string
	Path       string
	BaseBranch string
	// DefaultRuntime is the agent new launches of the workspace use when the task sets none. Nil means the
	// system agent. The provider only sets it while the agent still exists.
	DefaultRuntime *string
}

// BranchOption is a branch offered when a task chooses its base.
type BranchOption struct {
	Name    string
	Remote  bool
	Current bool
}
