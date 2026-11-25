// Package errors holds the failures of the workspace_branches module. Callers match them with errors.Is.
package errors

import "errors"

// ErrWorkspaceNotFound means the workspace of the request is not registered.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// ErrInvalidBranch means a branch reference is malformed or names something git must never receive as an option.
var ErrInvalidBranch = errors.New("invalid branch reference")

// ErrBranchNotFound means the reference is not one of the branches the workspace lists right now.
var ErrBranchNotFound = errors.New("branch not found")

// ErrAmbiguousBranch means more than one listed branch matches the reference, so it cannot be told which one is meant.
var ErrAmbiguousBranch = errors.New("ambiguous branch reference")
