//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"strings"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// SendMessage sends a message of the user to the agent. An idle agent receives it as a new prompt; a working agent
// receives it as a steer or as a follow-up, as the mode says, when the runtime supports that command.
type SendMessage struct {
	lifecycle *TaskLifecycle
	runtime   outbound.AgentRuntime
}

// NewSendMessage builds the use case on its ports.
func NewSendMessage(lifecycle *TaskLifecycle, runtime outbound.AgentRuntime) *SendMessage {
	return &SendMessage{lifecycle: lifecycle, runtime: runtime}
}

// Execute sends the message. The text is trimmed and the images are validated before anything else. A message with
// neither text nor images is rejected. It fails, changing nothing, when the task is missing, has no live agent, or
// the agent cannot queue the message while it works.
func (u *SendMessage) Execute(ctx context.Context, id models.TaskID, message string, images []models.ImageAttachment, mode models.MessageMode) error {
	text := strings.TrimSpace(message)
	validated, err := models.ValidateAllImages(images)
	if err != nil {
		return err
	}
	if text == "" && len(validated) == 0 {
		return werrors.EmptyMessage()
	}
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.AcceptsMessages() {
		return invalidState("enviar mensaje", task.Status)
	}
	command, err := commandFor(task.Status, mode, text, validated)
	if err != nil {
		return err
	}
	if err = u.ensureCanQueue(ctx, id, command); err != nil {
		return err
	}
	return u.runtime.Send(ctx, id, command)
}

// commandFor picks the command of a message: an idle agent always gets a prompt, a working one a steer or a follow-up.
func commandFor(status models.TaskStatus, mode models.MessageMode, message string, images []models.ImageAttachment) (models.AgentCommand, error) {
	if status == models.StatusIdle {
		return models.PromptCommand{Message: message, Images: images}, nil
	}
	switch mode {
	case models.MessageModeSteer:
		return models.SteerCommand{Message: message, Images: images}, nil
	case models.MessageModeFollowUp:
		return models.FollowUpCommand{Message: message, Images: images}, nil
	default:
		return nil, werrors.Unsupported("modo de mensaje desconocido")
	}
}

// ensureCanQueue checks that a steer or a follow-up can reach a running agent that supports it. Prompts need nothing.
func (u *SendMessage) ensureCanQueue(ctx context.Context, id models.TaskID, command models.AgentCommand) error {
	supports, refusal, queued := queueRequirement(command)
	if !queued {
		return nil
	}
	if !u.runtime.IsRunning(ctx, id) {
		return werrors.AgentNotRunning()
	}
	if !supports(u.runtime.Capabilities(id)) {
		return werrors.Unsupported(refusal)
	}
	return nil
}

// queueRequirement says which capability a command needs while the agent works, and the refusal text when it lacks it.
// The boolean is false for the commands that are never queued.
func queueRequirement(command models.AgentCommand) (func(models.AgentCapabilities) bool, string, bool) {
	switch command.(type) {
	case models.SteerCommand:
		return func(capabilities models.AgentCapabilities) bool { return capabilities.Steer },
			"el agente no permite redirigir (steer) mientras trabaja", true
	case models.FollowUpCommand:
		return func(capabilities models.AgentCapabilities) bool { return capabilities.FollowUp },
			"el agente no permite encolar mensajes (follow-up) mientras trabaja", true
	default:
		return nil, "", false
	}
}
