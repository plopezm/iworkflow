// Package errors holds the business errors of the data_home module. Their texts are for logs and
// tests; the settings screen shows translated messages chosen from these values.
package errors

import "errors"

var (
	// ErrEnvLocked is returned when IWF_HOME fixes the data folder, so the settings screen cannot move it.
	ErrEnvLocked = errors.New("the data folder is set by IWF_HOME")
	// ErrActiveTasks is returned when a task is starting, running or finishing.
	ErrActiveTasks = errors.New("tasks are starting, running or finishing")
	// ErrMovePending is returned when a move is already waiting for the next start.
	ErrMovePending = errors.New("a move is already pending")
	// ErrUnavailable is returned when the data folder cannot be resolved or its move state cannot be read.
	ErrUnavailable = errors.New("the data folder cannot be managed on this machine")
)

// DestinationReason names why a destination, or the current folder, cannot take the move.
type DestinationReason string

// Reasons a move is refused. They mirror the validation of the move engine.
const (
	ReasonRelative      DestinationReason = "relative"
	ReasonSame          DestinationReason = "same"
	ReasonNested        DestinationReason = "nested"
	ReasonNotEmpty      DestinationReason = "not_empty"
	ReasonNotWritable   DestinationReason = "not_writable"
	ReasonNoSpace       DestinationReason = "no_space"
	ReasonSourceUnsafe  DestinationReason = "source_unsafe"
	ReasonSourceMissing DestinationReason = "source_missing"
)

// DestinationError reports a move refused because of its destination or of the current folder.
type DestinationError struct {
	Reason DestinationReason
}

// Error names the reason; the screen translates it.
func (e *DestinationError) Error() string {
	return "data folder move refused: " + string(e.Reason)
}
