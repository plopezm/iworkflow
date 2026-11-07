package components_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// hookStripForTest is the strip of a task with one hook that is active and has failed, and one that is skipped for the
// task and has not run.
func hookStripForTest(output string) state.HookStrip {
	return state.NewHookStrip("abc123", "t1", []dto.HookStripDTO{
		{
			HookID: "h-1", Kind: "script", Description: "make test", Phase: "pre", Event: "task_start",
			Enabled: true, LastStatus: "failed", LastOutput: output, LastExitCode: 2,
			LastStartedAt: time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC),
		},
		{
			HookID: "h-2", Kind: "agent", Description: "run the linter", Phase: "post", Event: "task_finish",
			Enabled: true, Skipped: true, LastStatus: "none",
		},
	})
}

// The strip lists each hook with its phase, its status and the toggle that skips it for the task, which says the
// opposite of the current state.
func TestTaskHookStripShowsStatusAndToggleByState(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.TaskHookStrip(hookStripForTest("")))

	assert.Contains(t, body, `id="task-hooks-abc123"`, "the strip is the swap target of its toggles")
	assert.Contains(t, body, "make test")
	assert.Contains(t, body, "Before", "the pre phase is labelled")
	assert.Contains(t, body, "After", "the post phase is labelled")
	assert.Contains(t, body, ">Failed<", "a failed run shows its status")
	assert.Contains(t, body, ">Pending<", "a hook that has not run is pending")
	assert.Contains(t, body, `hx-post="/tasks/t1/hooks/h-1/skip?scope=abc123"`)
	assert.Contains(t, body, `hx-post="/tasks/t1/hooks/h-2/skip?scope=abc123"`)
	assert.Contains(t, body, `name="skipped" value="true"`, "an active hook is skipped by its toggle")
	assert.Contains(t, body, `name="skipped" value="false"`, "a skipped hook is included again by its toggle")
	assert.Contains(t, body, ">Skip<", "the active hook offers to skip")
	assert.Contains(t, body, ">Include<", "the skipped hook offers to include")
}

// The log is folded in a details element, and only a hook that ran has one.
func TestTaskHookStripFoldsTheLogOfARunHook(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.TaskHookStrip(hookStripForTest("compile failed")))

	assert.Contains(t, body, "<details", "a run hook has a log")
	assert.Contains(t, body, "<summary>Log</summary>")
	assert.Contains(t, body, "compile failed")
	assert.Contains(t, body, "2026-11-07 09:30", "the start of the run is shown to the minute")
	assert.Equal(t, 1, countOf(body, "<details"), "the hook that never ran has no log")
}

// The output is user-controlled text: it is escaped, never rendered as markup.
func TestTaskHookStripEscapesTheOutput(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.TaskHookStrip(hookStripForTest("<script>alert(1)</script>")))

	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "&lt;script&gt;alert(1)&lt;/script&gt;")
}

// A strip that could not be read says why in place of its rows.
func TestTaskHookStripShowsItsError(t *testing.T) {
	t.Parallel()

	strip := state.NewHookStrip("abc123", "t1", nil)
	strip.Error = "The hook could not be updated."

	body := renderIn(t, i18n.En, components.TaskHookStrip(strip))

	assert.Contains(t, body, "The hook could not be updated.")
	assert.NotContains(t, body, "data-task-hook=", "no row is shown without hooks")
}

// The strip labels follow the language of the page.
func TestTaskHookStripIsTranslated(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.Es, components.TaskHookStrip(hookStripForTest("")))

	assert.Contains(t, body, ">Omitir<")
	assert.Contains(t, body, ">Incluir<")
	assert.Contains(t, body, "Antes de")
	assert.Contains(t, body, "Después de")
}

// The actions tab shows the strip only when the view carries one.
func TestTaskActionsBodyShowsTheHookStripOnlyWhenPresent(t *testing.T) {
	t.Parallel()

	without := renderIn(t, i18n.En, components.TaskActionsBody(state.TaskDetailView{Scope: "abc123", Task: dto.TaskDTO{ID: "t1"}}))
	assert.NotContains(t, without, "task-hooks-abc123")

	view := state.TaskDetailView{Scope: "abc123", Task: dto.TaskDTO{ID: "t1"}}
	strip := hookStripForTest("")
	view.HookStrip = &strip
	with := renderIn(t, i18n.En, components.TaskActionsBody(view))
	assert.Contains(t, with, "task-hooks-abc123")
}

// countOf counts the occurrences of one fragment of markup.
func countOf(body string, fragment string) int {
	count := 0
	for i := 0; i+len(fragment) <= len(body); i++ {
		if body[i:i+len(fragment)] == fragment {
			count++
		}
	}
	return count
}

// The workspace menu links to the Hooks tab of the workspace, next to its files and its terminal.
func TestWorkspaceMenuLinksToTheHooksTab(t *testing.T) {
	t.Parallel()

	rows := state.SidebarRows{Workspaces: []state.SidebarWorkspace{{ID: "ws-1", Name: "Repo"}}}
	body := renderIn(t, i18n.En, components.SidebarBody(rows))

	assert.Contains(t, body, `href="/workspaces/ws-1/hooks"`)
	assert.Contains(t, body, ">Hooks</a>")
}

// A hook skipped for the task after it ran shows the status skipped, not the result of its last run, and keeps its
// log: the skip changes what the strip reports, not what the hook did.
func TestTaskHookStripShowsASkippedHookThatRanAsSkipped(t *testing.T) {
	t.Parallel()
	strip := state.NewHookStrip("abc123", "t1", []dto.HookStripDTO{{
		HookID: "h-1", Kind: "script", Description: "make test", Phase: "pre", Event: "task_start",
		Enabled: true, Skipped: true, LastStatus: "ok", LastOutput: "all green", LastExitCode: 0,
		LastStartedAt: time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC),
	}})

	body := renderIn(t, i18n.En, components.TaskHookStrip(strip))

	assert.Contains(t, body, ">Skipped<", "the skip is the status of the hook")
	assert.NotContains(t, body, ">Passed<", "the result of the last run is not shown while the hook is skipped")
	assert.Contains(t, body, ">Include<", "the skipped hook offers to include it again")
	assert.Contains(t, body, "all green", "the log of the run that happened stays available")
	assert.Equal(t, 1, countOf(body, "<details"))
}
