// Package errors holds the business errors of the task_hooks module.
package errors

import "errors"

// Sentinel errors. Callers test them with errors.Is.
var (
	// ErrInvalidHook means a field of the hook breaks an invariant.
	ErrInvalidHook = errors.New("invalid hook")
	// ErrHookNotFound means no hook has the requested id.
	ErrHookNotFound = errors.New("hook not found")
	// ErrRepository means the storage failed.
	ErrRepository = errors.New("hook storage failed")
)
