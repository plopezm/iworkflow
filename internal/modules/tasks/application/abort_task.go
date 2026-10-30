//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// AbortTask interrupts the turn the agent is running. The agent process stays alive.
type AbortTask struct {
	lifecycle *TaskLifecycle
	runtime   outbound.AgentRuntime
}

// NewAbortTask builds the use case on its ports.
func NewAbortTask(lifecycle *TaskLifecycle, runtime outbound.AgentRuntime) *AbortTask {
	return &AbortTask{lifecycle: lifecycle, runtime: runtime}
}

// Execute sends the abort to the live agent. It fails, sending nothing, when the task is missing or has no live agent.
func (u *AbortTask) Execute(ctx context.Context, id models.TaskID) error {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.HasLiveAgent() {
		return invalidState("abortar", task.Status)
	}
	return u.runtime.Send(ctx, id, models.AbortCommand{})
}
