// Package errors holds the business errors of the open_with module.
package errors

import "errors"

// Sentinel errors. Callers test them with errors.Is.
var (
	// ErrInvalidLauncher means a field of the launcher breaks an invariant.
	ErrInvalidLauncher = errors.New("invalid launcher")
	// ErrLauncherNotFound means no launcher has the requested id.
	ErrLauncherNotFound = errors.New("launcher not found")
	// ErrWorktreeNotFound means the task does not exist or has no worktree.
	ErrWorktreeNotFound = errors.New("the task has no worktree")
	// ErrLaunchFailed means the application could not be started.
	ErrLaunchFailed = errors.New("could not start the application")
	// ErrBrowseFailed means a folder could not be listed.
	ErrBrowseFailed = errors.New("could not read the folder")
	// ErrRepository means the storage failed.
	ErrRepository = errors.New("launcher storage failed")
)
