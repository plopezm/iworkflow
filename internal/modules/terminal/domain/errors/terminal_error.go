// Package errors holds the terminal business errors. Each sentinel is matched with errors.Is; the
// messages keep the exact text of the Rust version because the UI shows them.
package errors

import (
	"errors"
	"fmt"
)

// Sentinels. Their text is the message shown to the user.
var (
	ErrWorkspaceNotFound = errors.New("el workspace no existe")
	ErrTaskNotFound      = errors.New("la tarea no existe o no tiene worktree")
	ErrSessionNotFound   = errors.New("la sesión de terminal no existe")
	ErrInvalidSize       = errors.New("el tamaño del terminal debe ser mayor que cero")
	ErrPty               = errors.New("error del pseudo-terminal")
)

// Pty wraps a pseudo-terminal failure. The message is the cause.
func Pty(cause error) error {
	return fmt.Errorf("%w: %w", ErrPty, cause)
}
