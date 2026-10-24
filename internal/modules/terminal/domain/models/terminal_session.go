// Package models holds the terminal entities and value objects. They do not depend on infrastructure.
package models

import (
	"time"

	"github.com/google/uuid"
)

// TerminalStatus tells whether a session still has a live shell.
type TerminalStatus uint8

const (
	// TerminalStatusRunning means the shell still produces output.
	TerminalStatusRunning TerminalStatus = iota
	// TerminalStatusExited means the shell ended. The session stays listed until it is closed.
	TerminalStatusExited
)

// TerminalSession is the registry metadata of one terminal session.
type TerminalSession struct {
	ID TerminalSessionID
	// WorkspaceID is uuid.Nil for the home terminal, which belongs to no workspace.
	WorkspaceID uuid.UUID
	// WorkspaceName is captured when the session opens and is never updated afterwards.
	WorkspaceName string
	CreatedAt     time.Time
	Status        TerminalStatus
}
