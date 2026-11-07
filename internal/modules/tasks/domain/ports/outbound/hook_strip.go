package outbound

import (
	"context"
	"time"
)

// Status values of a StripHook.LastStatus. None means the hook has not run for the task yet.
const (
	StripStatusNone    = "none"
	StripStatusRunning = "running"
	StripStatusOK      = "ok"
	StripStatusFailed  = "failed"
	StripStatusTimeout = "timeout"
	StripStatusError   = "error"
	StripStatusSkipped = "skipped"
)

// HookStrip is the status strip of the hooks of a task, shown in its actions tab. It is optional and consumer-side:
// the composition root implements it with the task_hooks module, so tasks never imports task_hooks. When it is not
// set, the actions tab shows no strip.
type HookStrip interface {
	// State returns every hook of the workspace of the task, in position order, with its status for the task.
	State(ctx context.Context, workspaceID string, taskID string) ([]StripHook, error)
	// SetSkipped disables a hook of the workspace for one task, or enables it again. A hook of another workspace is refused.
	SetSkipped(ctx context.Context, workspaceID string, taskID string, hookID string, skipped bool) error
}

// StripHook is one hook of the workspace of a task as the strip shows it. Kind is "script" or "agent"; Description is
// the command of a script or the prompt of an agent. Phase is "pre" or "post" and Event is "task_start" or
// "task_finish". LastStatus is one of the StripStatus values. The last run fields are zero when the hook has not run.
type StripHook struct {
	HookID        string
	Kind          string
	Description   string
	Phase         string
	Event         string
	Enabled       bool
	Skipped       bool
	LastStatus    string
	LastOutput    string
	LastExitCode  int
	LastStartedAt time.Time
}
