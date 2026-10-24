//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package pi

import (
	"context"
	"slices"
	"time"

	"iworkflow/internal/core/env"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime"
)

const (
	// interactiveHostEnv enables the dialogs (ask_user_*) and the activity widgets in gentle-shell.
	interactiveHostEnv = "GENTLE_SHELL_INTERACTIVE_HOST"
	resumeFlag         = "--continue"
	providerFlag       = "--provider"
	modelFlag          = "--model"
	thinkingFlag       = "--thinking"
	errAlreadyRunning  = "el agente ya está en ejecución"
)

// RpcRuntimeAdapter launches and controls pi agents that speak the RPC protocol (`<program> --mode rpc`),
// one process per task.
type RpcRuntimeAdapter struct {
	process   *runtime.ProcessRuntime
	baseArgs  []string
	subagents *subagentStore
}

var _ outbound.AgentRuntime = (*RpcRuntimeAdapter)(nil)

// NewRpcRuntimeAdapter builds the real runtime: `<program> --mode rpc`.
func NewRpcRuntimeAdapter(program string) *RpcRuntimeAdapter {
	return NewRpcRuntimeAdapterWithCommand(program, rpcArgs())
}

// NewRpcRuntimeAdapterWithCommand runs an arbitrary command that speaks the RPC protocol (useful in tests).
func NewRpcRuntimeAdapterWithCommand(program string, baseArgs []string) *RpcRuntimeAdapter {
	return &RpcRuntimeAdapter{
		process:   runtime.NewProcessRuntime(program),
		baseArgs:  slices.Clone(baseArgs),
		subagents: newSubagentStore(),
	}
}

// WithStopTimeout sets the graceful-stop budget before the process is forced.
func (a *RpcRuntimeAdapter) WithStopTimeout(timeout time.Duration) *RpcRuntimeAdapter {
	a.process = a.process.WithStopTimeout(timeout)
	return a
}

// rpcArgs returns the arguments that put pi in RPC mode. It returns a fresh slice on every call.
func rpcArgs() []string {
	return []string{"--mode", "rpc"}
}

// launchArgs returns the agent arguments: the base ones, the resume flag, the provider and model, and the
// thinking level. The base slice is never modified.
func launchArgs(baseArgs []string, resume bool, options models.LaunchOptions) []string {
	args := slices.Clone(baseArgs)
	if resume {
		args = append(args, resumeFlag)
	}
	args = append(args, modelArgs(options)...)
	return append(args, thinkingArgs(options)...)
}

func modelArgs(options models.LaunchOptions) []string {
	if options.Model == nil {
		return nil
	}
	return []string{providerFlag, options.Model.Provider, modelFlag, options.Model.Model}
}

func thinkingArgs(options models.LaunchOptions) []string {
	if options.Thinking == nil {
		return nil
	}
	return []string{thinkingFlag, *options.Thinking}
}

// capabilities declares what pi supports: every command and option except modes, which pi does not expose.
func capabilities() models.AgentCapabilities {
	caps := models.FullAgentCapabilities()
	caps.Modes = false
	return caps
}

// Start launches the pi process of the task. A start discards the subagents of the previous run.
func (a *RpcRuntimeAdapter) Start(ctx context.Context, taskID models.TaskID, cwd string, resume bool, options models.LaunchOptions, sink inbound.AgentEventSink) error {
	if a.process.IsRunning(taskID) {
		return werrors.Runtime(errAlreadyRunning)
	}
	a.subagents.remove(taskID)
	spec := runtime.LaunchSpec{
		AgentArgs: launchArgs(a.baseArgs, resume, options),
		ExtraEnv:  []env.Var{{Key: interactiveHostEnv, Value: "1"}},
	}
	return a.process.Start(ctx, taskID, cwd, spec, options.Limits, newPiCodec(a.subagents), sink)
}

// Send delivers a command to the pi process of the task.
func (a *RpcRuntimeAdapter) Send(ctx context.Context, taskID models.TaskID, command models.AgentCommand) error {
	return a.process.Send(ctx, taskID, command)
}

// Stop stops the pi process of the task and waits for it to end.
func (a *RpcRuntimeAdapter) Stop(ctx context.Context, taskID models.TaskID) error {
	return a.process.Stop(ctx, taskID)
}

// StopAll stops every live pi process. It is best effort.
func (a *RpcRuntimeAdapter) StopAll(ctx context.Context) {
	a.process.StopAll(ctx)
}

// IsRunning reports whether the pi process of the task is alive.
func (a *RpcRuntimeAdapter) IsRunning(_ context.Context, taskID models.TaskID) bool {
	return a.process.IsRunning(taskID)
}

// Subagents returns the last known snapshot of the subagents of the task, kept after the process ends.
func (a *RpcRuntimeAdapter) Subagents(_ context.Context, taskID models.TaskID) []models.SubagentTask {
	return a.subagents.get(taskID)
}

// Capabilities returns the capabilities of pi for any task.
func (a *RpcRuntimeAdapter) Capabilities(_ models.TaskID) models.AgentCapabilities {
	return capabilities()
}

// RuntimeCapabilities returns the capabilities of pi, whichever runtime was requested.
func (a *RpcRuntimeAdapter) RuntimeCapabilities(_ *models.RuntimeID) models.AgentCapabilities {
	return capabilities()
}
