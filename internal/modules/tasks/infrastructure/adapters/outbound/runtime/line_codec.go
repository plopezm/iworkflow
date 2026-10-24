package runtime

import (
	"iworkflow/internal/core/env"
	"iworkflow/internal/modules/tasks/domain/models"
)

// LineCodec is the protocol seam of the process runtime. Each agent protocol (pi, claude, cursor)
// provides one: it says what each stdout line means and which stdin lines a command turns into.
// ProcessRuntime calls it from one goroutine per direction, serialized by a mutex, so a codec keeps
// per-process state without locking of its own.
type LineCodec interface {
	// Decode turns one stdout line (without its line ending) into domain events. Called in line order.
	Decode(taskID models.TaskID, line string) []models.AgentEvent
	// Encode returns the stdin line (without its line ending) for a command. requestID is unique per
	// command and lets protocols correlate requests and replies. ok false means the command was accepted
	// but produces no line yet (for example, other questions are still unanswered).
	Encode(command models.AgentCommand, requestID string) (line string, ok bool, err error)
	// Start returns the lines written to stdin right after the process starts (a handshake, for
	// example). Called once.
	Start() []string
	// TakeOutgoing returns lines the codec produced on its own, in reaction to a decode or an encode
	// (replies to requests of the agent, the next handshake step, held commands). Called after each
	// Decode and each Encode, and its lines are written in order.
	TakeOutgoing() []string
}

// NoHandshake supplies the empty Start and TakeOutgoing for codecs that need neither. Embed it.
type NoHandshake struct{}

// Start has no lines to write at launch.
func (NoHandshake) Start() []string { return nil }

// TakeOutgoing has no lines of its own.
func (NoHandshake) TakeOutgoing() []string { return nil }

// LaunchSpec says how one agent program is launched.
type LaunchSpec struct {
	// AgentArgs are the arguments of the agent program.
	AgentArgs []string
	// ExtraEnv are variables added to the session environment. They override it per key.
	ExtraEnv []env.Var
}
