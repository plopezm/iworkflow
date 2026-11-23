package models

import "github.com/google/uuid"

// WorkspaceTarget is a workspace resolved to open a terminal: its name and color (snapshots) and its
// working directory. Color is a palette key; empty means no color.
type WorkspaceTarget struct {
	Name  string
	Color string
	Path  string
}

// TaskTarget is a task resolved to open a terminal: the workspace it belongs to, its title
// (snapshot) and its working directory (the worktree).
type TaskTarget struct {
	WorkspaceID uuid.UUID
	Name        string
	Path        string
}
