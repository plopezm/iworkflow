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
	// WorkspaceName and WorkspaceColor are captured when the session opens and are never updated afterwards.
	// The color is a palette key; empty means no color.
	WorkspaceName  string
	WorkspaceColor string
	CreatedAt      time.Time
	Status         TerminalStatus
}
