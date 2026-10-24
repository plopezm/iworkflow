package state

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// statusVariant is the color of a request, pipeline or job state. Unknown states are muted.
func statusVariant(status string) string {
	switch status {
	case "success", "passed", "merged", "mergeable", "can_be_merged":
		return "ok"
	case "failed", "cannot_be_merged", "conflict", "broken_status":
		return "error"
	case "running", "pending", "created", "preparing", "waiting_for_resource", "scheduled",
		"checking", "ci_still_running", "opened":
		return "info"
	case "manual", "blocked_status", "not_approved", "discussions_not_resolved":
		return "warn"
	default:
		return "muted"
	}
}

// statusLabel is the translated label of a state. Unknown states are shown as they arrive.
func statusLabel(ctx context.Context, status string) string {
	msg, known := statusMessage(status)
	if !known {
		return status
	}
	return msg.T(ctx)
}

// statusMessage returns the text of a known state.
func statusMessage(status string) (i18n.Msg, bool) {
	switch status {
	case "success":
		return msgStatusSuccess, true
	case "failed":
		return msgStatusFailed, true
	case "running":
		return msgStatusRunning, true
	case "pending":
		return msgStatusPending, true
	case "created":
		return msgStatusCreated, true
	case "canceled":
		return msgStatusCanceled, true
	case "skipped":
		return msgStatusSkipped, true
	case "manual":
		return msgStatusManual, true
	case "opened":
		return msgStatusOpened, true
	case "merged":
		return msgStatusMerged, true
	case "closed":
		return msgStatusClosed, true
	case "mergeable", "can_be_merged":
		return msgStatusMergeable, true
	case "cannot_be_merged":
		return msgStatusCannotMerge, true
	case "conflict":
		return msgStatusConflict, true
	case "ci_still_running":
		return msgStatusCIRunning, true
	case "ci_must_pass":
		return msgStatusCIMustPass, true
	case "not_approved":
		return msgStatusNotApproved, true
	case "discussions_not_resolved":
		return msgStatusDiscussions, true
	case "checking":
		return msgStatusChecking, true
	default:
		return i18n.Msg{}, false
	}
}
