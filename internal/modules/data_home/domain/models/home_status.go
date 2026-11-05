// Package models holds the entities of the data_home module: where the data folder comes from, the
// move waiting for the next start and the outcome of the last move. It is pure Go: no I/O.
package models

import "time"

// HomeSource says which rule picked the data folder.
type HomeSource string

// The three sources, in precedence order: the environment, the per-user pointer, the default folder.
const (
	SourceEnv     HomeSource = "env"
	SourcePointer HomeSource = "pointer"
	SourceDefault HomeSource = "default"
)

// PendingMove is a move requested from the settings screen that the next start will carry out.
type PendingMove struct {
	From string
	To   string
}

// MoveOutcome is the result of the last move, as the start-up recorded it.
type MoveOutcome struct {
	OK         bool
	From       string
	To         string
	Error      string
	Warnings   []string
	FinishedAt time.Time
}

// HomeStatus is the data folder as the settings screen shows it. EnvLocked is true when IWF_HOME
// fixes the folder, in which case the screen is read-only.
type HomeStatus struct {
	Current   string
	Source    HomeSource
	EnvLocked bool
	Pending   *PendingMove
	Last      *MoveOutcome
}
