// Package errors holds the business errors of the memories module.
package errors

import "errors"

// ErrMemoryNotFound marks a memory that does not exist for the workspace.
var ErrMemoryNotFound = errors.New("memory not found")

// ErrWorkspaceNotFound marks a workspace that is not registered.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// ErrTaskNotFound marks a task that does not exist.
var ErrTaskNotFound = errors.New("task not found")
