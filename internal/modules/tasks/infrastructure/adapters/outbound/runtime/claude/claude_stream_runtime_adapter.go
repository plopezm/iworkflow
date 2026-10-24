//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package claude

import (
	"context"
	"time"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime"
)

const (
	resumeFlag = "--continue"
	modelFlag  = "--model"
	effortFlag = "--effort"
)

// streamJSONArgs puts claude in non-interactive mode, with stream-json input and output, and leaves the
// permissions to the host (control_request/can_use_tool). File edits are accepted (acceptEdits); other
// tools ask for confirmation. It returns a fresh slice on every call.
func streamJSONArgs() []string {
	return []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--replay-user-messages",
		"--permission-prompt-tool", "stdio",
		"--permission-mode", "acceptEdits",
	}
}

// StreamRuntimeAdapter launches and controls claude processes (`claude -p` with stream-json), one per task.
type StreamRuntimeAdapter struct {
	process  *runtime.ProcessRuntime
	baseArgs []string
}

var _ outbound.AgentRuntime = (*StreamRuntimeAdapter)(nil)

// NewStreamRuntimeAdapter builds the real runtime: `<program> -p --input-format stream-json ...`.
func NewStreamRuntimeAdapter(program string) *StreamRuntimeAdapter {
	return NewStreamRuntimeAdapterWithCommand(program, streamJSONArgs())
}

// NewStreamRuntimeAdapterWithCommand runs an arbitrary command that speaks stream-json (useful in tests).
func NewStreamRuntimeAdapterWithCommand(program string, baseArgs []string) *StreamRuntimeAdapter {
	return &StreamRuntimeAdapter{
		process:  runtime.NewProcessRuntime(program),
		baseArgs: baseArgs,
	}
}

// WithStopTimeout sets the graceful-stop budget before the process is forced.
func (a *StreamRuntimeAdapter) WithStopTimeout(timeout time.Duration) *StreamRuntimeAdapter {
	a.process = a.process.WithStopTimeout(timeout)
	return a
}

// launchArgs returns the agent arguments: the base ones, the resume flag, the model (an alias or a full
// name; the provider does not apply to Claude) and the effort level. The base slice is never modified.
func launchArgs(baseArgs []string, resume bool, options models.LaunchOptions) []string {
	args := make([]string, 0, len(baseArgs)+len(resumeFlag)+2)
	args = append(args, baseArgs...)
	if resume {
		args = append(args, resumeFlag)
	}
	if options.Model != nil {
		args = append(args, modelFlag, options.Model.Model)
	}
	return append(args, effortArgs(options)...)
}

// effortArgs maps the thinking level to `--effort`, which Claude accepts as low|medium|high|xhigh|max.
// "off" has no flag, and "minimal" (which only pi has) becomes the lowest level Claude has.
func effortArgs(options models.LaunchOptions) []string {
	if options.Thinking == nil || *options.Thinking == "off" {
		return nil
	}
	level := *options.Thinking
	if level == "minimal" {
		level = "low"
	}
	return []string{effortFlag, level}
}

// capabilities declares what claude supports: dialogs, model and effort selection, resume and modes.
// Steering, follow-up and subagent activity are not declared.
func capabilities() models.AgentCapabilities {
	return models.AgentCapabilities{
		Dialogs:        true,
		ModelSelection: true,
		ThinkingLevels: true,
		Resume:         true,
		Modes:          true,
	}
}

// Start launches the claude process of the task. resume continues the previous session of the folder.
func (a *StreamRuntimeAdapter) Start(ctx context.Context, taskID models.TaskID, cwd string, resume bool, options models.LaunchOptions, sink inbound.AgentEventSink) error {
	spec := runtime.LaunchSpec{AgentArgs: launchArgs(a.baseArgs, resume, options)}
	return a.process.Start(ctx, taskID, cwd, spec, options.Limits, newStreamProtocol(), sink)
}

// Send delivers a command to the claude process of the task.
func (a *StreamRuntimeAdapter) Send(ctx context.Context, taskID models.TaskID, command models.AgentCommand) error {
	return a.process.Send(ctx, taskID, command)
}

// Stop stops the claude process of the task and waits for it to end.
func (a *StreamRuntimeAdapter) Stop(ctx context.Context, taskID models.TaskID) error {
	return a.process.Stop(ctx, taskID)
}

// StopAll stops every live claude process. It is best effort.
func (a *StreamRuntimeAdapter) StopAll(ctx context.Context) {
	a.process.StopAll(ctx)
}

// IsRunning reports whether the claude process of the task is alive.
func (a *StreamRuntimeAdapter) IsRunning(_ context.Context, taskID models.TaskID) bool {
	return a.process.IsRunning(taskID)
}

// Subagents is always empty: claude does not expose subagent activity through this protocol.
func (a *StreamRuntimeAdapter) Subagents(_ context.Context, _ models.TaskID) []models.SubagentTask {
	return nil
}

// Capabilities returns the capabilities of claude for any task.
func (a *StreamRuntimeAdapter) Capabilities(_ models.TaskID) models.AgentCapabilities {
	return capabilities()
}

// RuntimeCapabilities returns the capabilities of claude, whichever runtime was requested.
func (a *StreamRuntimeAdapter) RuntimeCapabilities(_ *models.RuntimeID) models.AgentCapabilities {
	return capabilities()
}
