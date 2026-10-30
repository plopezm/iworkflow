package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
	inbound "iworkflow/internal/modules/tasks/domain/ports/inbound"
)

// AgentRuntime controls the coding agents, one per task. The port does not assume how each
// implementation talks to its agent. Declared for slice F.
type AgentRuntime interface {
	// Start launches the agent in cwd and delivers its events to sink. resume means "continue the
	// previous session of that folder, when the agent supports it"; otherwise a new session starts.
	Start(ctx context.Context, taskID models.TaskID, cwd string, resume bool, options models.LaunchOptions, sink inbound.AgentEventSink) error
	// Send delivers a command to the agent of the task. Commands the agent does not declare in its
	// capabilities may be rejected with an Unsupported error.
	Send(ctx context.Context, taskID models.TaskID, command models.AgentCommand) error
	// Stop stops the agent in an orderly way (forcing it when it takes too long) and waits for it to
	// end. When it returns, the ProcessExited event has already reached the sink.
	Stop(ctx context.Context, taskID models.TaskID) error
	// StopAll stops every live agent (application shutdown). It is best effort: a failure stopping one
	// does not prevent stopping the others.
	StopAll(ctx context.Context)
	// IsRunning reports whether the agent of the task is alive.
	IsRunning(ctx context.Context, taskID models.TaskID) bool
	// Subagents returns the last known state of the subagents of the task (kept after the agent ends).
	// Empty when the task is unknown or the runtime does not declare subagents.
	Subagents(ctx context.Context, taskID models.TaskID) []models.SubagentTask
	// Capabilities returns the capabilities of the agent of the task; minimal for an unknown task.
	Capabilities(taskID models.TaskID) models.AgentCapabilities
	// RuntimeCapabilities returns the capabilities of the runtime a launch with runtime would use (nil
	// means the default). An unknown runtime yields the minimal set. Launch and resume use them to
	// strip what the runtime does not support before calling Start.
	RuntimeCapabilities(runtime *models.RuntimeID) models.AgentCapabilities
}
