package models

import "github.com/google/uuid"

// TerminalSessionID identifies a terminal session.
type TerminalSessionID uuid.UUID

// NewTerminalSessionID returns a new random identifier.
func NewTerminalSessionID() TerminalSessionID {
	return TerminalSessionID(uuid.New())
}

// TerminalSessionIDFromUUID wraps an existing UUID.
func TerminalSessionIDFromUUID(id uuid.UUID) TerminalSessionID {
	return TerminalSessionID(id)
}

// UUID returns the underlying UUID.
func (id TerminalSessionID) UUID() uuid.UUID {
	return uuid.UUID(id)
}

// String renders the canonical UUID text.
func (id TerminalSessionID) String() string {
	return uuid.UUID(id).String()
}
