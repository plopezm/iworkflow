// Package errors holds the business errors of the workspace_groups module.
package errors

import "errors"

// Sentinel errors. Callers test them with errors.Is.
var (
	// ErrInvalidGroup means a field of the group breaks an invariant.
	ErrInvalidGroup = errors.New("invalid group")
	// ErrGroupNotFound means no group has the requested id.
	ErrGroupNotFound = errors.New("group not found")
	// ErrWorkspaceNotFound means the workspace to group or pin does not exist.
	ErrWorkspaceNotFound = errors.New("workspace not found")
	// ErrRepository means the storage failed.
	ErrRepository = errors.New("group storage failed")
)
