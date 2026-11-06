//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical across runtimes.
package agy

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime"
)

// acpArgs puts `agy` in ACP server mode: JSON-RPC over stdio. It returns a fresh slice on every call.
func acpArgs() []string {
	return []string{"acp"}
}

// AgyAcpRuntimeAdapter launches and controls the Antigravity CLI agent (`agy acp`), one process per task.
//
// The session id of each task is kept in sessionsDir/<task> and reopened with session/load when the task resumes.
type AgyAcpRuntimeAdapter struct {
	process     *runtime.ProcessRuntime
	baseArgs    []string
	sessionsDir string
}

var _ outbound.AgentRuntime = (*AgyAcpRuntimeAdapter)(nil)

// NewAgyAcpRuntimeAdapter builds the real runtime: `<program> acp`.
func NewAgyAcpRuntimeAdapter(program string, sessionsDir string) *AgyAcpRuntimeAdapter {
	return NewAgyAcpRuntimeAdapterWithCommand(program, acpArgs(), sessionsDir)
}

// NewAgyAcpRuntimeAdapterWithCommand runs an arbitrary command that speaks ACP (useful in tests).
func NewAgyAcpRuntimeAdapterWithCommand(program string, baseArgs []string, sessionsDir string) *AgyAcpRuntimeAdapter {
	return &AgyAcpRuntimeAdapter{
		process:     runtime.NewProcessRuntime(program),
		baseArgs:    baseArgs,
		sessionsDir: sessionsDir,
	}
}

// WithStopTimeout sets the graceful-stop budget before the process is forced.
func (a *AgyAcpRuntimeAdapter) WithStopTimeout(timeout time.Duration) *AgyAcpRuntimeAdapter {
	a.process = a.process.WithStopTimeout(timeout)
	return a
}

// capabilitiesSet declares what agy supports: dialogs, model selection, resume and modes.
func capabilitiesSet() models.AgentCapabilities {
	return models.AgentCapabilities{
		Dialogs:        true,
		ModelSelection: true,
		Resume:         true,
		Modes:          true,
	}
}

// sessionFile is where the session id of the task is saved.
func (a *AgyAcpRuntimeAdapter) sessionFile(taskID models.TaskID) string {
	return filepath.Join(a.sessionsDir, taskID.String())
}

// readSession returns the session saved in file, when there is one and it is not blank.
func readSession(file string) (string, bool) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	trimmed := strings.TrimSpace(string(raw))
	return trimmed, trimmed != ""
}

// saveSession writes the session id to file, creating its folder when needed. A failure is only logged: the
// session keeps running, it just cannot be resumed.
func saveSession(file string, sessionID string) {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		slog.Warn("no se pudo guardar la sesión de agy", "file", file, "error", err)
		return
	}
	if err := os.WriteFile(file, []byte(sessionID), 0o644); err != nil {
		slog.Warn("no se pudo guardar la sesión de agy", "file", file, "error", err)
	}
}

// Start launches the agy agent of the task. resume reopens the session saved for the task, when there is one.
func (a *AgyAcpRuntimeAdapter) Start(ctx context.Context, taskID models.TaskID, cwd string, resume bool, options models.LaunchOptions, sink inbound.AgentEventSink) error {
	sessionFile := a.sessionFile(taskID)
	config := acpSessionConfig{cwd: cwd}
	if resume {
		if saved, ok := readSession(sessionFile); ok {
			config.resumeSession = &saved
		}
	}
	if options.Model != nil {
		modelID := options.Model.Model
		config.model = &modelID
	}
	protocol := newACPProtocol(config).withSessionListener(func(sessionID string) {
		saveSession(sessionFile, sessionID)
	})
	spec := runtime.LaunchSpec{AgentArgs: slices.Clone(a.baseArgs)}
	return a.process.Start(ctx, taskID, cwd, spec, options.Limits, protocol, sink)
}

// Send delivers a command to the agy agent of the task.
func (a *AgyAcpRuntimeAdapter) Send(ctx context.Context, taskID models.TaskID, command models.AgentCommand) error {
	return a.process.Send(ctx, taskID, command)
}

// Stop stops the agy agent of the task and waits for it to end.
func (a *AgyAcpRuntimeAdapter) Stop(ctx context.Context, taskID models.TaskID) error {
	return a.process.Stop(ctx, taskID)
}

// StopAll stops every live agy agent. It is best effort.
func (a *AgyAcpRuntimeAdapter) StopAll(ctx context.Context) {
	a.process.StopAll(ctx)
}

// IsRunning reports whether the agy agent of the task is alive.
func (a *AgyAcpRuntimeAdapter) IsRunning(_ context.Context, taskID models.TaskID) bool {
	return a.process.IsRunning(taskID)
}

// Subagents returns empty: subagent tracking is internal to the CLI session.
func (a *AgyAcpRuntimeAdapter) Subagents(_ context.Context, _ models.TaskID) []models.SubagentTask {
	return nil
}

// Capabilities returns the capabilities of agy for any task.
func (a *AgyAcpRuntimeAdapter) Capabilities(_ models.TaskID) models.AgentCapabilities {
	return capabilitiesSet()
}

// RuntimeCapabilities returns the capabilities of agy, whichever runtime was requested.
func (a *AgyAcpRuntimeAdapter) RuntimeCapabilities(_ *models.RuntimeID) models.AgentCapabilities {
	return capabilitiesSet()
}
