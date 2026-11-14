package errors

import "errors"

// Failures of opening a path that the agent wrote in the chat.
var (
	// ErrOpenPathInvalid is a path that cannot be resolved: it is empty, it holds a NUL byte or it is relative and the
	// task has no worktree.
	ErrOpenPathInvalid = errors.New("the path cannot be opened")
	// ErrOpenPathBlocked is a path of a kind that is never launched from the chat (programs and scripts that run).
	ErrOpenPathBlocked = errors.New("this kind of file is not opened from the chat")
	// ErrOpenPathMissing is a path that does not exist on disk.
	ErrOpenPathMissing = errors.New("the path does not exist")
)
