package dto

import (
	"errors"

	"github.com/google/uuid"

	"iworkflow/internal/modules/terminal/domain/models"
)

// Validation messages of the identifiers received in the paths. They reach the user as they are,
// as in the Rust version.
var (
	ErrInvalidWorkspaceID = errors.New("id de workspace inválido")
	ErrInvalidTaskID      = errors.New("id de tarea inválido")
	ErrInvalidSessionID   = errors.New("id de sesión inválido")
)

// ParseWorkspaceID validates the workspace id of a path.
func ParseWorkspaceID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, ErrInvalidWorkspaceID
	}
	return id, nil
}

// ParseTaskID validates the task id of a path.
func ParseTaskID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, ErrInvalidTaskID
	}
	return id, nil
}

// ParseSessionID validates the session id of a path.
func ParseSessionID(raw string) (models.TerminalSessionID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return models.TerminalSessionID{}, ErrInvalidSessionID
	}
	return models.TerminalSessionIDFromUUID(id), nil
}
