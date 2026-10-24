package models

import "time"

// Comment is a comment of a Jira issue. Comments are read live from Jira; they are not imported.
type Comment struct {
	ID        string
	Author    string
	Body      string
	CreatedAt time.Time
}

// Transition is a status change Jira currently allows for an issue.
type Transition struct {
	ID   string
	Name string
	// ToStatus is the status the issue ends up in.
	ToStatus string
}

// WorkspaceOption is a workspace an issue can be converted into a task of.
type WorkspaceOption struct {
	ID         string
	Name       string
	BaseBranch string
}

// BranchOption is a branch a task worktree can start from.
type BranchOption struct {
	Name   string
	Remote bool
}
