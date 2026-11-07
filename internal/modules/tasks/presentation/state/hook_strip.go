package state

import (
	"time"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// HookStatus is the badge a hook shows on the strip of a task.
type HookStatus string

// Values of HookStatus. A hook that has not run for the task is pending.
const (
	HookStatusPending HookStatus = "pending"
	HookStatusRunning HookStatus = "running"
	HookStatusOK      HookStatus = "ok"
	HookStatusFailed  HookStatus = "failed"
	HookStatusTimeout HookStatus = "timeout"
	HookStatusError   HookStatus = "error"
	HookStatusSkipped HookStatus = "skipped"
)

// hookDescriptionMaxChars is the longest description the strip shows; a longer one is cut and ends with an ellipsis.
const hookDescriptionMaxChars = 80

// HookStripRow is one hook of the status strip of a task. ToggleSkipped is the value the skip toggle posts: true skips
// the hook for this task, false includes it again. Output is the log of the latest run, shown only when HasRun is true.
type HookStripRow struct {
	HookID        string
	Kind          string
	Description   string
	Phase         string
	Event         string
	Enabled       bool
	Skipped       bool
	ToggleSkipped bool
	Status        HookStatus
	HasRun        bool
	Output        string
	ExitCode      int
	StartedAt     string
}

// HookStrip is the status strip of the hooks of a task, shown in its actions tab. Scope is the page instance, as in
// TaskDetailView. Error says why the strip could not be read or updated; it is shown in place of the rows.
type HookStrip struct {
	Scope  string
	TaskID string
	Rows   []HookStripRow
	Error  string
}

// NewHookStrip builds the strip of a task from its hooks, in the order given.
func NewHookStrip(scope string, taskID string, hooks []dto.HookStripDTO) HookStrip {
	rows := make([]HookStripRow, 0, len(hooks))
	for _, hook := range hooks {
		rows = append(rows, newHookStripRow(hook))
	}
	return HookStrip{Scope: scope, TaskID: taskID, Rows: rows}
}

// Visible reports whether the strip has anything to show: a hook, or the reason it could not be read.
func (s HookStrip) Visible() bool {
	return len(s.Rows) > 0 || s.Error != ""
}

// newHookStripRow maps one hook of the strip to its row.
func newHookStripRow(hook dto.HookStripDTO) HookStripRow {
	return HookStripRow{
		HookID:        hook.HookID,
		Kind:          hook.Kind,
		Description:   truncateRunes(hook.Description, hookDescriptionMaxChars),
		Phase:         hook.Phase,
		Event:         hook.Event,
		Enabled:       hook.Enabled,
		Skipped:       hook.Skipped,
		ToggleSkipped: !hook.Skipped,
		Status:        hookStatusOf(hook.LastStatus),
		HasRun:        hook.LastStatus != "none",
		Output:        hook.LastOutput,
		ExitCode:      hook.LastExitCode,
		StartedAt:     startedClock(hook.LastStartedAt),
	}
}

// startedClock is the start of a run shown to the minute in UTC. A hook that has not run has no start.
func startedClock(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(hookStartedLayout)
}

// hookStatusOf maps the status of the latest run to the badge. A status the strip does not know is an error, so it
// is never shown as a success.
func hookStatusOf(status string) HookStatus {
	switch status {
	case "none":
		return HookStatusPending
	case "running":
		return HookStatusRunning
	case "ok":
		return HookStatusOK
	case "failed":
		return HookStatusFailed
	case "timeout":
		return HookStatusTimeout
	case "skipped":
		return HookStatusSkipped
	default:
		return HookStatusError
	}
}

// hookStartedLayout is the layout of the start of a run: the date and the minute.
const hookStartedLayout = "2006-01-02 15:04"

// truncateRunes cuts value to max characters, ending with an ellipsis when it had to cut.
func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}
