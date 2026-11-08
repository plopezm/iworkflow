package models

// HookEvent is the task action a lifecycle hook is attached to.
type HookEvent string

// Lifecycle hook events. Only the start and the finish of a task run hooks in v1.
const (
	HookEventStart  HookEvent = "start"
	HookEventFinish HookEvent = "finish"
)

// HookPhase tells whether a lifecycle hook runs before or after the task action.
type HookPhase string

// Lifecycle hook phases.
const (
	HookPhasePre  HookPhase = "pre"
	HookPhasePost HookPhase = "post"
)

// HookResult is the outcome of the task action, given to the post hooks.
type HookResult string

// Lifecycle hook results.
const (
	HookResultOK      HookResult = "ok"
	HookResultFailed  HookResult = "failed"
	HookResultAborted HookResult = "aborted"
)

// HookRequest is one run of the lifecycle hooks of a task action. Result is only set for the post phase.
// WorkspacePath is the folder of the task's workspace; the task carries the worktree, branch and title.
type HookRequest struct {
	Event         HookEvent
	Phase         HookPhase
	Result        HookResult
	Task          Task
	WorkspacePath string
}

// HookReport is what a run of the lifecycle hooks reports besides its error: the warnings of the hooks
// whose failure does not block the action.
type HookReport struct {
	Warnings []string
}
