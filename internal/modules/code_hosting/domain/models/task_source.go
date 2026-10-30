package models

// TaskSource is what the code hosting module needs to know about a task; the composition root
// provides it from the tasks module.
type TaskSource struct {
	// RepoPath is the root of the workspace repository, where `origin` lives.
	RepoPath string
	// Branch is the task branch, which is also the source branch of the merge request.
	Branch string
	// BaseBranch is the branch the merge request targets (the base of the workspace or task).
	BaseBranch  string
	Title       string
	Description string
}
