//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package cursor

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

// acpArgs puts `agent` in ACP server mode: JSON-RPC over stdio. It returns a fresh slice on every call.
func acpArgs() []string {
	return []string{"acp"}
}

// CursorAcpRuntimeAdapter launches and controls Cursor's agent (`agent acp`), one process per task.
//
// ACP cannot continue "the last session of a folder" (session/list does not answer), so the session id of each
// task is kept in sessionsDir/<task> and reopened with session/load when the task resumes.
type CursorAcpRuntimeAdapter struct {
	process     *runtime.ProcessRuntime
	baseArgs    []string
	sessionsDir string
}

var _ outbound.AgentRuntime = (*CursorAcpRuntimeAdapter)(nil)

// NewCursorAcpRuntimeAdapter builds the real runtime: `<program> acp`.
func NewCursorAcpRuntimeAdapter(program string, sessionsDir string) *CursorAcpRuntimeAdapter {
	return NewCursorAcpRuntimeAdapterWithCommand(program, acpArgs(), sessionsDir)
}

// NewCursorAcpRuntimeAdapterWithCommand runs an arbitrary command that speaks ACP (useful in tests).
func NewCursorAcpRuntimeAdapterWithCommand(program string, baseArgs []string, sessionsDir string) *CursorAcpRuntimeAdapter {
	return &CursorAcpRuntimeAdapter{
		process:     runtime.NewProcessRuntime(program),
		baseArgs:    baseArgs,
		sessionsDir: sessionsDir,
	}
}

// WithStopTimeout sets the graceful-stop budget before the process is forced.
func (a *CursorAcpRuntimeAdapter) WithStopTimeout(timeout time.Duration) *CursorAcpRuntimeAdapter {
	a.process = a.process.WithStopTimeout(timeout)
	return a
}

// capabilitiesSet declares what Cursor supports: dialogs, model selection, resume and modes. Steering, follow-up
// and subagent activity are not declared.
func capabilitiesSet() models.AgentCapabilities {
	return models.AgentCapabilities{
		Dialogs:        true,
		ModelSelection: true,
		Resume:         true,
		Modes:          true,
	}
}

// sessionFile is where the session id of the task is saved.
func (a *CursorAcpRuntimeAdapter) sessionFile(taskID models.TaskID) string {
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
		slog.Warn("no se pudo guardar la sesión de Cursor", "file", file, "error", err)
		return
	}
	if err := os.WriteFile(file, []byte(sessionID), 0o644); err != nil {
		slog.Warn("no se pudo guardar la sesión de Cursor", "file", file, "error", err)
	}
}

// Start launches the Cursor agent of the task. resume reopens the session saved for the task, when there is one.
func (a *CursorAcpRuntimeAdapter) Start(ctx context.Context, taskID models.TaskID, cwd string, resume bool, options models.LaunchOptions, sink inbound.AgentEventSink) error {
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

// Send delivers a command to the Cursor agent of the task.
func (a *CursorAcpRuntimeAdapter) Send(ctx context.Context, taskID models.TaskID, command models.AgentCommand) error {
	return a.process.Send(ctx, taskID, command)
}

// Stop stops the Cursor agent of the task and waits for it to end.
func (a *CursorAcpRuntimeAdapter) Stop(ctx context.Context, taskID models.TaskID) error {
	return a.process.Stop(ctx, taskID)
}

// StopAll stops every live Cursor agent. It is best effort.
func (a *CursorAcpRuntimeAdapter) StopAll(ctx context.Context) {
	a.process.StopAll(ctx)
}

// IsRunning reports whether the Cursor agent of the task is alive.
func (a *CursorAcpRuntimeAdapter) IsRunning(_ context.Context, taskID models.TaskID) bool {
	return a.process.IsRunning(taskID)
}

// Subagents is always empty: Cursor's ACP does not expose subagent activity through this protocol.
func (a *CursorAcpRuntimeAdapter) Subagents(_ context.Context, _ models.TaskID) []models.SubagentTask {
	return nil
}

// Capabilities returns the capabilities of Cursor for any task.
func (a *CursorAcpRuntimeAdapter) Capabilities(_ models.TaskID) models.AgentCapabilities {
	return capabilitiesSet()
}

// RuntimeCapabilities returns the capabilities of Cursor, whichever runtime was requested.
func (a *CursorAcpRuntimeAdapter) RuntimeCapabilities(_ *models.RuntimeID) models.AgentCapabilities {
	return capabilitiesSet()
}
