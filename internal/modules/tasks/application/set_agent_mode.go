//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// SetAgentMode switches a running agent between agent and plan mode.
type SetAgentMode struct {
	lifecycle *TaskLifecycle
	runtime   outbound.AgentRuntime
}

// NewSetAgentMode builds the use case on its ports.
func NewSetAgentMode(lifecycle *TaskLifecycle, runtime outbound.AgentRuntime) *SetAgentMode {
	return &SetAgentMode{lifecycle: lifecycle, runtime: runtime}
}

// Execute sends the mode to the live agent. It fails, sending nothing, when the task is missing, has no live agent,
// the mode is unknown, or the runtime does not switch modes.
func (u *SetAgentMode) Execute(ctx context.Context, id models.TaskID, mode models.AgentMode) error {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.HasLiveAgent() {
		return invalidState("cambiar el modo", task.Status)
	}
	if _, known := models.ParseAgentMode(string(mode)); !known {
		return werrors.Unsupported("modo de agente desconocido: " + string(mode))
	}
	if !u.runtime.Capabilities(id).Modes {
		return werrors.Unsupported("el agente no permite cambiar de modo")
	}
	return u.runtime.Send(ctx, id, models.SetModeCommand{Mode: mode})
}
