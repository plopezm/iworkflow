package outbound

import "context"

// AgentSessionRequest describes one ephemeral agent session. Dir is the worktree of the task. Runtime, Model,
// Thinking and Profile are the optional overrides of the hook; empty means the workspace default.
type AgentSessionRequest struct {
	Dir      string
	Prompt   string
	Runtime  string
	Model    string
	Thinking string
	Profile  string
}

// AgentSessionResult is what the agent said in its last turn. Failed covers an agent error, a process that
// exited before the turn settled, and a dialog nobody could answer.
type AgentSessionResult struct {
	Text          string
	Failed        bool
	FailureReason string
}

// AgentSession runs one ephemeral agent session in a directory: no task row, no graph node. It returns when the
// turn settles or the agent fails. A session that outlives ctx returns the ctx error wrapped; the session is
// always stopped before Run returns. An error means the session could not be attempted.
type AgentSession interface {
	Run(ctx context.Context, request AgentSessionRequest) (AgentSessionResult, error)
}
