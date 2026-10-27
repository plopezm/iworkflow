package components

import (
	"context"
	"strconv"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// subagentStatusLabel is the readable state of a subagent. The statuses are the ones the runtimes report.
func subagentStatusLabel(ctx context.Context, status string) string {
	switch status {
	case "queued":
		return msgStatusQueued.T(ctx)
	case "running":
		return msgStatusRunning.T(ctx)
	case "waiting":
		return msgStatusWaiting.T(ctx)
	case "completed":
		return msgStatusCompleted.T(ctx)
	case "failed":
		return msgStatusFailed.T(ctx)
	case "cancelled":
		return msgStatusCancelled.T(ctx)
	case "timed_out":
		return msgStatusTimedOut.T(ctx)
	default:
		return msgStatusUnknown.T(ctx)
	}
}

// subagentStatusVariant is the colour of the badge of a subagent state.
func subagentStatusVariant(status string) string {
	switch status {
	case "running":
		return "info"
	case "waiting", "timed_out":
		return "warn"
	case "completed":
		return "ok"
	case "failed":
		return "error"
	default:
		return "muted"
	}
}

// subagentDisplayName is the agent of a subagent and, when it has one, its label.
func subagentDisplayName(subagent dto.SubagentDTO) string {
	if subagent.Label != nil && *subagent.Label != "" {
		return subagent.Agent + " · " + *subagent.Label
	}
	return subagent.Agent
}

// subagentActivity summarises the work of a subagent: its turns and its tool calls.
func subagentActivity(ctx context.Context, turns uint32, tools uint32) string {
	return msgActivitySummary.Fmt(ctx, map[string]string{
		"turns": strconv.FormatUint(uint64(turns), 10),
		"tools": strconv.FormatUint(uint64(tools), 10),
	})
}

// subagentRowClass is the row of a subagent in the list; the selected one is highlighted.
func subagentRowClass(selected bool) string {
	if selected {
		return "stack w-full gap-1 rounded-lg border border-border/60 bg-surface-2 p-2 text-left transition-colors"
	}
	return "stack w-full gap-1 rounded-lg border border-border/60 p-2 text-left transition-colors hover:bg-surface-2"
}

// subagentAriaPressed is the pressed state of a row, read by assistive technology.
func subagentAriaPressed(selected bool) string {
	if selected {
		return "true"
	}
	return "false"
}
