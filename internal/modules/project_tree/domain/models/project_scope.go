package models

import (
	"strings"

	"github.com/google/uuid"
)

// taskPrefix starts the wire form of a task scope (`task:<uuid>`).
const taskPrefix string = "task:"

type scopeKind uint8

const (
	scopeWorkspace scopeKind = iota + 1
	scopeTask
)

// ProjectScope says what a project tree is rooted at: a workspace directory or a task worktree.
type ProjectScope struct {
	kind scopeKind
	id   uuid.UUID
}

// WorkspaceScope is the scope of a workspace directory.
func WorkspaceScope(id uuid.UUID) ProjectScope {
	return ProjectScope{kind: scopeWorkspace, id: id}
}

// TaskScope is the scope of a task worktree.
func TaskScope(id uuid.UUID) ProjectScope {
	return ProjectScope{kind: scopeTask, id: id}
}

// ParseProjectScope reads the wire form: a bare uuid is a workspace, `task:<uuid>` is a task
// worktree. It reports false for anything else.
func ParseProjectScope(value string) (ProjectScope, bool) {
	if rest, isTask := strings.CutPrefix(value, taskPrefix); isTask {
		id, err := uuid.Parse(rest)
		if err != nil {
			return ProjectScope{}, false
		}
		return TaskScope(id), true
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return ProjectScope{}, false
	}
	return WorkspaceScope(id), true
}

// ID returns the identifier of the workspace or task.
func (s ProjectScope) ID() uuid.UUID {
	return s.id
}

// IsTask reports whether the scope is a task worktree rather than a workspace.
func (s ProjectScope) IsTask() bool {
	return s.kind == scopeTask
}

// Wire returns the wire form of the scope: a bare uuid for a workspace, `task:<uuid>` for a task.
// It is the value the scope travels as in URLs and forms, and ParseProjectScope reads it back.
func (s ProjectScope) Wire() string {
	if s.kind == scopeTask {
		return taskPrefix + s.id.String()
	}
	return s.id.String()
}
