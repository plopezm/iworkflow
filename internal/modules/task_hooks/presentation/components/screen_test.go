package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/task_hooks/presentation/components"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// renderIn renders the component in the locale, as the screen is rendered for a request.
func renderIn(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

// The library shows the hooks, their assignment count and their latest error, which opens the history of the hook.
func TestScreenShowsTheLibraryRows(t *testing.T) {
	t.Parallel()
	view := state.Screen{
		Tab: state.TabLibrary,
		Hooks: []state.HookRow{{
			ID: "h-1", Name: "lint <now>", Kind: "script", Assignments: 2,
			LastError: &state.LastError{Status: state.ResultFailed, StartedAt: "2026-11-07 09:30", HistoryPath: state.HistoryPath("h-1", "", true)},
		}},
	}

	body := renderIn(t, components.Screen(view))

	assert.Contains(t, body, `data-hook="h-1"`)
	assert.Contains(t, body, "lint &lt;now&gt;", "the name is escaped")
	assert.Contains(t, body, "2 assignments")
	assert.Contains(t, body, "Last error · Failed · 2026-11-07 09:30")
	assert.Contains(t, body, `hx-get="/hooks/history?applied=1&amp;failed=1&amp;hook=h-1&amp;project="`)
}

// A failure of the last action is shown with its reason, and the notice of a success as a badge.
func TestScreenShowsTheNoticeAndTheFailure(t *testing.T) {
	t.Parallel()
	body := renderIn(t, components.Screen(state.Screen{Tab: state.TabLibrary, Notice: state.NoticeHookDeleted}))
	assert.Contains(t, body, "Hook deleted.")

	failed := renderIn(t, components.Screen(state.Screen{Tab: state.TabLibrary, Failure: "invalid hook: the name is required"}))
	assert.Contains(t, failed, "The action failed: invalid hook: the name is required")
}

// The assignments of the open hook carry their values in the toggle, so the toggle posts the stored values back, and a
// disabled assignment offers to enable it.
func TestScreenAssignmentsToggleCarriesTheStoredValues(t *testing.T) {
	t.Parallel()
	form := state.AssignmentForm{ID: "a-1", Project: "ws-1", Event: "task_start", Phase: "pre", OnFailure: "warn", Enabled: false}
	view := state.Screen{Tab: state.TabLibrary, Selected: &state.HookDetail{
		HookID: "h-1", Name: "lint", Assignments: []state.AssignmentRow{{Form: form, ProjectName: "alpha"}},
		Add: state.AssignmentForm{Event: "task_start", Phase: "pre", OnFailure: "block"}, Test: state.AssignmentForm{Event: "task_start", Phase: "pre"},
	}}

	body := renderIn(t, components.Screen(view))

	assert.Contains(t, body, `data-assignment="a-1"`)
	assert.Contains(t, body, `hx-post="/hooks/h-1/assignments/a-1"`)
	assert.Contains(t, body, `&#34;project&#34;:&#34;ws-1&#34;`, "the toggle posts the project of the assignment")
	assert.Contains(t, body, `&#34;enabled&#34;:&#34;on&#34;`, "the enable button posts the flag that turns the assignment on")
	assert.Contains(t, body, ">Enable<")
	assert.Contains(t, body, "Policy: Warn and continue")
}

// The run order moves only where the group has a neighbour; the ends are disabled.
func TestScreenGroupDisablesTheMovesAtTheEnds(t *testing.T) {
	t.Parallel()
	view := state.Screen{Tab: state.TabLibrary, Group: &state.Group{
		Project: "ws-1", Event: "task_start", Phase: "pre",
		Rows: []state.GroupRow{
			{ID: "a1", HookName: "first", Enabled: true, Policy: "block", DownOrder: "a2,a1"},
			{ID: "a2", HookName: "second", Enabled: true, Policy: "block", UpOrder: "a2,a1"},
		},
	}}

	body := renderIn(t, components.Screen(view))

	assert.Contains(t, body, `data-group-row="a1"`)
	assert.Contains(t, body, `disabled`, "the first row cannot move up")
	assert.Contains(t, body, `hx-vals="{&#34;event&#34;:&#34;task_start&#34;,&#34;order&#34;:&#34;a2,a1&#34;,&#34;phase&#34;:&#34;pre&#34;,&#34;project&#34;:&#34;ws-1&#34;}"`)
}

// The history shows the failed-only box checked by default, the runs with their output folded, and every value escaped.
func TestScreenHistoryShowsTheRunsWithTheirOutput(t *testing.T) {
	t.Parallel()
	view := state.Screen{Tab: state.TabHistory, History: &state.History{
		Failed: true, Hooks: []state.Choice{{ID: "h-1", Name: "lint"}}, Projects: []state.Choice{{ID: "ws-1", Name: "alpha"}},
		Runs: []state.RunRow{{
			ID: "r-1", HookName: "lint", TaskID: "t1", ProjectName: "alpha", Status: "failed", ExitCode: 2, HasExitCode: true,
			Output: "<script>boom</script>", StartedAt: "2026-11-07 09:30",
		}},
	}}

	body := renderIn(t, components.Screen(view))

	assert.Contains(t, body, `data-run="r-1"`)
	assert.Contains(t, body, "exit code 2")
	assert.Contains(t, body, "&lt;script&gt;boom&lt;/script&gt;", "the output is escaped")
	assert.NotContains(t, body, "<script>boom</script>")
	assert.Contains(t, body, `name="failed" value="1" checked`)
	assert.Contains(t, body, "alpha · Task t1 · 2026-11-07 09:30")
}

// A run with no exit code, such as an agent run, shows no exit code.
func TestScreenHistoryOmitsAnExitCodeWhenThereIsNone(t *testing.T) {
	t.Parallel()
	view := state.Screen{Tab: state.TabHistory, History: &state.History{Runs: []state.RunRow{{
		ID: "r-agent", HookName: "review", Status: "failed", ExitCode: -1, HasExitCode: false, Output: "no verdict",
	}}}}

	body := renderIn(t, components.Screen(view))

	assert.NotContains(t, body, "exit code")
	assert.Contains(t, body, "no verdict")
}

// A script hook has no shell switch: the runner decides it from the command. The command says so, and Browse stays.
func TestScriptEditorHasNoShellSwitch(t *testing.T) {
	t.Parallel()
	view := state.Screen{Tab: state.TabLibrary, Editor: &state.HookForm{Kind: "script", Timeout: "60", Command: "codegraph init"}}

	body := renderIn(t, components.Screen(view))

	assert.NotContains(t, body, "use_shell")
	assert.NotContains(t, body, "Run through a shell")
	assert.Contains(t, body, "A program or a full command line")
	assert.NotContains(t, body, "browse_project", "the browser starts at the home folder, not at a project")
	assert.NotContains(t, body, "Project used to browse folders")
	assert.Contains(t, body, `data-hook-editor`)
	assert.Contains(t, body, `name="command"`)
}

// The advanced options are folded while empty and unfolded when one holds a value or the folder browser is on the
// working directory, so a typed value is never hidden.
func TestScriptEditorFoldsTheAdvancedOptionsOnlyWhenEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		form    state.HookForm
		browser *state.Browser
		open    bool
	}{
		"empty":       {form: state.HookForm{Kind: "script", Timeout: "60"}, open: false},
		"args":        {form: state.HookForm{Kind: "script", Timeout: "60", Args: "init"}, open: true},
		"env":         {form: state.HookForm{Kind: "script", Timeout: "60", Env: "A=1"}, open: true},
		"working dir": {form: state.HookForm{Kind: "script", Timeout: "60", WorkingDir: "sub"}, open: true},
		"browsing":    {form: state.HookForm{Kind: "script", Timeout: "60"}, browser: &state.Browser{Field: "working_dir", Path: "/tmp"}, open: true},
		"browsing the command": {
			form: state.HookForm{Kind: "script", Timeout: "60"}, browser: &state.Browser{Field: "command", Path: "/tmp", Files: true}, open: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			form := tc.form
			body := renderIn(t, components.Screen(state.Screen{Tab: state.TabLibrary, Editor: &form, Browser: tc.browser}))

			assert.Contains(t, body, "Advanced options")
			assert.Contains(t, body, "Extra parameters")
			if tc.open {
				assert.Regexp(t, `<details[^>]* open`, body)
			} else {
				assert.NotRegexp(t, `<details[^>]* open`, body)
			}
		})
	}
}

// The agent kind has none of the script fields.
func TestAgentEditorHasNoScriptFields(t *testing.T) {
	t.Parallel()
	body := renderIn(t, components.Screen(state.Screen{Tab: state.TabLibrary, Editor: &state.HookForm{Kind: "agent", Timeout: "60"}}))

	assert.NotContains(t, body, "Advanced options")
	assert.NotContains(t, body, `name="command"`)
}

// The run order filter lays its three selects and the button out as a grid, so the selects are not squeezed.
func TestGroupFilterIsAGrid(t *testing.T) {
	t.Parallel()
	view := state.Screen{Tab: state.TabLibrary, Group: &state.Group{Project: "ws-1", Event: "task_start", Phase: "pre"}}

	body := renderIn(t, components.Screen(view))

	assert.Contains(t, body, "Run order")
	assert.Regexp(t, `<form class="grid [^"]*"[^>]*hx-get="/hooks/group`, body)
}

// The history filter lays the two selects and the failed-only switch out in one grid, the switch in a box as tall as
// the selects so the three line up.
func TestHistoryFilterAlignsTheFailedOnlySwitchWithTheSelects(t *testing.T) {
	t.Parallel()
	view := state.Screen{Tab: state.TabHistory, History: &state.History{Failed: true}}

	body := renderIn(t, components.Screen(view))

	assert.Regexp(t, `<form class="grid [^"]*"[^>]*data-history-filter`, body)
	assert.Regexp(t, `<span class="input row [^"]*">\s*<input type="checkbox" name="failed" value="1" checked`, body)
	assert.Contains(t, body, "Failures only")
}
