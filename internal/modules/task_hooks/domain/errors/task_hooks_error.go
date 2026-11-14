// Package errors holds the business errors of the task_hooks module.
package errors

import "errors"

// Sentinel errors. Callers test them with errors.Is.
var (
	// ErrInvalidHook means a field of the hook breaks an invariant.
	ErrInvalidHook = errors.New("invalid hook")
	// ErrHookNotFound means no hook has the requested id.
	ErrHookNotFound = errors.New("hook not found")
	// ErrAssignmentNotFound means no assignment has the requested id.
	ErrAssignmentNotFound = errors.New("hook assignment not found")
	// ErrRepository means the storage failed.
	ErrRepository = errors.New("hook storage failed")
)

// ErrRunNotStarted means a hook could not even be attempted (bad template, missing directory, no executable).
var ErrRunNotStarted = errors.New("could not start the hook")

// ErrRunCancelled means the hook was started and then stopped because the context that owned it was cancelled.
var ErrRunCancelled = errors.New("the hook was cancelled")

// ErrHookBlocked means a hook with the block policy failed, so the task action must not continue.
var ErrHookBlocked = errors.New("a hook blocked the action")
