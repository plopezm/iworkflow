package models

import "github.com/google/uuid"

// WorkspaceTarget is a workspace resolved to open a terminal: its name (snapshot) and working directory.
type WorkspaceTarget struct {
	Name string
	Path string
}

// TaskTarget is a task resolved to open a terminal: the workspace it belongs to, its title
// (snapshot) and its working directory (the worktree).
type TaskTarget struct {
	WorkspaceID uuid.UUID
	Name        string
	Path        string
}
