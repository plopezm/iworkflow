// Package models holds the tasks entities and value objects. It is pure Go: no I/O.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"strconv"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// TaskStatus is the lifecycle state of a task. Its string value is the one persisted.
type TaskStatus string

// Lifecycle states of a task.
const (
	// StatusBacklog is a task created without worktree or agent, waiting to start.
	StatusBacklog TaskStatus = "backlog"
	// StatusStarting means the worktree is created and the agent process is starting.
	StatusStarting TaskStatus = "starting"
	// StatusRunning means the agent is working.
	StatusRunning TaskStatus = "running"
	// StatusIdle means the agent finished its run and waits for instructions.
	StatusIdle TaskStatus = "idle"
	// StatusFinishing means the agent is being closed and changes consolidated.
	StatusFinishing TaskStatus = "finishing"
	// StatusAwaitingReview means changes are ready for manual review.
	StatusAwaitingReview TaskStatus = "awaiting_review"
	// StatusMerged means integrated into the base branch; the worktree is gone.
	StatusMerged TaskStatus = "merged"
	// StatusDiscarded means discarded; worktree and branch are gone.
	StatusDiscarded TaskStatus = "discarded"
	// StatusConflict means the merge produced conflicts; the worktree is kept.
	StatusConflict TaskStatus = "conflict"
	// StatusFailed means launching or operating the task failed.
	StatusFailed TaskStatus = "failed"
	// StatusInterrupted means the agent process ended unexpectedly (for example a server restart).
	StatusInterrupted TaskStatus = "interrupted"
	// StatusRateLimited means the provider rejected the agent for a usage limit; it resumes on renewal.
	StatusRateLimited TaskStatus = "rate_limited"
)

// AllTaskStatuses lists every status in declaration order.
var AllTaskStatuses = [...]TaskStatus{
	StatusBacklog,
	StatusStarting,
	StatusRunning,
	StatusIdle,
	StatusFinishing,
	StatusAwaitingReview,
	StatusMerged,
	StatusDiscarded,
	StatusConflict,
	StatusFailed,
	StatusInterrupted,
	StatusRateLimited,
}

// ParseTaskStatus converts the persisted name back to a status; unknown names are a repository error.
func ParseTaskStatus(value string) (TaskStatus, error) {
	for _, status := range AllTaskStatuses {
		if string(status) == value {
			return status, nil
		}
	}
	return "", werrors.Repository("estado desconocido: " + value)
}

// HasLiveAgent reports whether an agent process is (or should be) alive.
func (s TaskStatus) HasLiveAgent() bool {
	return s == StatusStarting || s == StatusRunning || s == StatusIdle
}

// IsClosed reports the final states: the worktree no longer exists.
func (s TaskStatus) IsClosed() bool {
	return s == StatusMerged || s == StatusDiscarded
}

// HasWorktree reports whether the worktree exists on disk (not pending start and not closed).
func (s TaskStatus) HasWorktree() bool {
	return !s.IsInactive()
}

// AcceptsMessages reports whether the agent can receive messages.
func (s TaskStatus) AcceptsMessages() bool {
	return s.HasLiveAgent()
}

// AfterAgentEvent returns the status an agent event provokes, with its error text, when it applies.
// The boolean is false when the event does not change the status.
func (s TaskStatus) AfterAgentEvent(event AgentEvent) (TaskStatus, *string, bool) {
	var next TaskStatus
	var detail *string
	switch typed := event.(type) {
	case RunStarted:
		if !s.HasLiveAgent() {
			return "", nil, false
		}
		next = StatusRunning
	case RunSettled:
		if s != StatusStarting && s != StatusRunning {
			return "", nil, false
		}
		next = StatusIdle
	case UsageLimitReached:
		if !s.HasLiveAgent() {
			return "", nil, false
		}
		next = StatusRateLimited
		message := typed.Message
		detail = &message
	case ProcessExited:
		if !s.HasLiveAgent() {
			return "", nil, false
		}
		next = StatusInterrupted
		message := "el agente terminó inesperadamente (" + exitDetail(typed.Code) + ")"
		detail = &message
	default:
		return "", nil, false
	}
	if next == s {
		return "", nil, false
	}
	return next, detail, true
}

func exitDetail(code *int32) string {
	if code == nil {
		return "sin código"
	}
	return "código " + strconv.FormatInt(int64(*code), 10)
}

// CanFinish reports whether the task can be finished (its agent closed and changes consolidated).
func (s TaskStatus) CanFinish() bool {
	switch s {
	case StatusRunning, StatusIdle, StatusInterrupted, StatusFailed, StatusConflict:
		return true
	default:
		return false
	}
}

// CanResume reports whether a stopped task can be resumed.
func (s TaskStatus) CanResume() bool {
	switch s {
	case StatusInterrupted, StatusRateLimited, StatusFailed, StatusConflict, StatusAwaitingReview:
		return true
	default:
		return false
	}
}

// CanMerge reports whether the task can be integrated into its base branch.
func (s TaskStatus) CanMerge() bool {
	return s == StatusAwaitingReview || s == StatusConflict
}

// CanDiscard reports whether the task can be discarded.
func (s TaskStatus) CanDiscard() bool {
	return !s.IsClosed() && s != StatusFinishing && s != StatusBacklog
}

// CanReset reports whether the task can be reset to backlog. Backlog has nothing to reset, a merge
// cannot be undone, and finishing has an operation in progress.
func (s TaskStatus) CanReset() bool {
	return s != StatusBacklog && s != StatusMerged && s != StatusFinishing
}

// CanStart reports whether only a backlog task can be started.
func (s TaskStatus) CanStart() bool {
	return s == StatusBacklog
}

// CanEdit reports whether only a backlog task can change its title or prompt.
func (s TaskStatus) CanEdit() bool {
	return s == StatusBacklog
}

// CanDelete reports whether only a backlog task can be physically deleted (not discarded).
func (s TaskStatus) CanDelete() bool {
	return s == StatusBacklog
}

// IsInactive reports tasks without a live agent and without a pending worktree or branch. They can
// be deleted in cascade with their workspace.
func (s TaskStatus) IsInactive() bool {
	return s == StatusBacklog || s == StatusMerged || s == StatusDiscarded
}

// String returns the persisted name.
func (s TaskStatus) String() string {
	return string(s)
}

// TaskStage is the aggregated view of a task status used by the kanban board.
type TaskStage uint8

// Board stages.
const (
	// StageBacklog is a task not started yet.
	StageBacklog TaskStage = iota + 1
	// StageWorking means the agent works without pending dialogs.
	StageWorking
	// StageAttention means the user must intervene: pending dialog, error or interruption.
	StageAttention
	// StageReview means changes are ready for manual review.
	StageReview
	// StageDone means integrated or discarded.
	StageDone
)
