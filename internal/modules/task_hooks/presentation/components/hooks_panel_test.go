package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/task_hooks/presentation/components"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

func renderPanel(t *testing.T, locale i18n.Locale, view state.View) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale})
	var out bytes.Buffer
	require.NoError(t, components.Panel(view).Render(ctx, &out))
	return out.String()
}

// tagOf returns the opening tag that contains marker, so a test can check one attribute of one element.
func tagOf(t *testing.T, body string, marker string) string {
	t.Helper()
	at := strings.Index(body, marker)
	require.NotEqual(t, -1, at, "marker %q not rendered", marker)
	start := strings.LastIndex(body[:at], "<")
	end := strings.Index(body[at:], ">")
	require.NotEqual(t, -1, end)
	return body[start : at+end+1]
}

func oneRow(id string) state.HookRow {
	return state.HookRow{
		ID: id, Kind: "script", Enabled: true, OnFailure: "block", Timeout: "120", Summary: "./check.sh --fast",
		UpOrder: "a,b", DownOrder: "b,a",
	}
}

func viewWith(rows ...state.HookRow) state.View {
	return state.View{
		WorkspaceID: "w1",
		Sections: []state.EventSection{{
			Event:  "task_start",
			Before: state.PhaseGroup{Phase: "pre", Rows: rows, NewPath: "/workspaces/w1/hooks/editor?event=task_start&phase=pre"},
			After:  state.PhaseGroup{Phase: "post", NewPath: "/workspaces/w1/hooks/editor?event=task_start&phase=post"},
		}},
	}
}

// The tab shows the two groups of each event with the Spanish labels, and every hook with its actions.
func TestPanelGroupsHooksByEventAndPhaseInSpanish(t *testing.T) {
	t.Parallel()
	body := renderPanel(t, i18n.Es, viewWith(oneRow("h1")))

	assert.Contains(t, body, "Antes de")
	assert.Contains(t, body, "Después de")
	assert.Contains(t, body, `data-hooks-group="pre"`)
	assert.Contains(t, body, `data-hooks-group="post"`)
	assert.Contains(t, body, `data-hook="h1"`)
	assert.Contains(t, body, "./check.sh --fast")
	assert.Contains(t, body, "Probar")
	assert.Contains(t, body, "Tiempo máximo: 120 s")
	assert.Contains(t, body, "Bloquea la acción")
	assert.Contains(t, body, `hx-post="/workspaces/w1/hooks/h1/test"`)
	assert.Contains(t, body, `hx-post="/workspaces/w1/hooks/h1/delete"`)
	assert.Contains(t, body, `hx-confirm="¿Eliminar este hook?"`)
}

// Moving a hook submits the whole order it would have; the move that has no neighbour is disabled.
func TestPanelDisablesTheMoveThatHasNoNeighbour(t *testing.T) {
	t.Parallel()
	first := oneRow("a")
	first.UpOrder = ""
	body := renderPanel(t, i18n.En, viewWith(first, oneRow("b")))

	up := tagOf(t, body, `aria-label="Move up"`)
	assert.Contains(t, up, "disabled")
	assert.NotContains(t, tagOf(t, body, `hx-vals="{&#34;order&#34;:&#34;b,a`), "disabled")
	down := tagOf(t, body, `aria-label="Move down"`)
	assert.NotContains(t, down, "disabled")
	assert.Contains(t, down, `hx-post="/workspaces/w1/hooks/reorder"`)
}

// The output of a test is user content: it is escaped, never rendered as markup.
func TestPanelEscapesTheOutputOfATest(t *testing.T) {
	t.Parallel()
	row := oneRow("h1")
	row.Test = &state.TestResult{Status: "failed", ExitCode: 3, Output: "<script>alert(1)</script>"}

	body := renderPanel(t, i18n.En, viewWith(row))

	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.Contains(t, body, "Failed · exit code 3")
	assert.Contains(t, body, `data-hook-test="failed"`)
}

func TestPanelShowsTheAgentTestWithoutAnExitCode(t *testing.T) {
	t.Parallel()
	row := oneRow("h1")
	row.Kind = "agent"
	row.Test = &state.TestResult{Status: "ok", ExitCode: -1, Output: ""}

	body := renderPanel(t, i18n.En, viewWith(row))

	assert.Contains(t, body, "Agent")
	assert.Contains(t, body, "Passed")
	assert.NotContains(t, body, "exit code")
	assert.Contains(t, body, "(no output)")
}

// The tab shows the notice of the last action, the failure of an action, and the empty state of a group in Spanish.
func TestPanelShowsTheNoticeTheFailureAndTheEmptyGroups(t *testing.T) {
	t.Parallel()
	view := viewWith()
	view.Notice = state.NoticeSaved
	view.Failure = "invalid hook: the command is required"

	body := renderPanel(t, i18n.Es, view)

	assert.Contains(t, body, "Hook guardado.")
	assert.Contains(t, body, "invalid hook: the command is required")
	assert.Contains(t, body, "Ningún hook todavía.")
}

// The editor offers the fields of the kind it has: a script its command, an agent its prompt.
func TestEditorShowsTheFieldsOfTheSelectedKind(t *testing.T) {
	t.Parallel()
	script := viewWith()
	script.Editor = &state.HookForm{Event: "task_start", Phase: "pre", Kind: "script", Enabled: true, Timeout: "600", Command: "make"}
	body := renderPanel(t, i18n.En, script)

	assert.Contains(t, body, `name="command"`)
	assert.Contains(t, body, `name="use_shell"`)
	assert.NotContains(t, body, `name="prompt"`)
	assert.Contains(t, body, `name="kind" value="script"`)
	assert.Contains(t, body, `hx-get="/workspaces/w1/hooks/editor"`)
	assert.Contains(t, body, `hx-vals="{&#34;kind&#34;:&#34;agent&#34;}"`)
	assert.Contains(t, body, `hx-include="closest form"`)
	assert.Contains(t, body, `hx-post="/workspaces/w1/hooks"`)

	agent := viewWith()
	agent.Editor = &state.HookForm{ID: "h9", Event: "task_finish", Phase: "post", Kind: "agent", Prompt: "Review it", Profile: "fast"}
	body = renderPanel(t, i18n.En, agent)

	assert.Contains(t, body, `name="prompt"`)
	assert.Contains(t, body, `name="profile"`)
	assert.NotContains(t, body, `name="command"`)
	assert.Contains(t, body, `name="id" value="h9"`)
	assert.Contains(t, body, "Edit hook")
}

// The kind-specific values are escaped in the editor too, so a prompt with markup stays text.
func TestEditorEscapesWhatWasTyped(t *testing.T) {
	t.Parallel()
	view := viewWith()
	view.Editor = &state.HookForm{Event: "task_start", Phase: "pre", Kind: "script", Command: `"><script>x</script>`}

	body := renderPanel(t, i18n.En, view)

	assert.NotContains(t, body, "<script>x</script>")
	assert.Contains(t, body, `value="&#34;&gt;&lt;script&gt;x&lt;/script&gt;"`)
}

// Each notice of an action has its own text, in the language of the request.
func TestPanelShowsTheTextOfEachNotice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		notice state.Notice
		want   string
	}{
		{notice: state.NoticeSaved, want: "Hook saved."},
		{notice: state.NoticeUpdated, want: "Hook updated."},
		{notice: state.NoticeDeleted, want: "Hook deleted."},
		{notice: state.NoticeReordered, want: "Order saved."},
	}
	for _, tt := range tests {
		t.Run(string(tt.notice), func(t *testing.T) {
			t.Parallel()
			view := viewWith()
			view.Notice = tt.notice

			body := renderPanel(t, i18n.En, view)

			assert.Contains(t, body, tt.want)
		})
	}
}

// Each status of a test run has its own text and its own marker; a run that exited shows its exit code, and an agent
// has none to show.
func TestPanelShowsTheTextOfEachTestStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   state.TestResult
		want     string
		wantExit string
	}{
		{name: "passed", result: state.TestResult{Status: state.ResultOK, ExitCode: 0}, want: "Passed", wantExit: "exit code 0"},
		{name: "failed", result: state.TestResult{Status: state.ResultFailed, ExitCode: 3}, want: "Failed", wantExit: "exit code 3"},
		{name: "timed out", result: state.TestResult{Status: state.ResultTimeout, ExitCode: -1}, want: "Timed out"},
		{name: "could not run", result: state.TestResult{Status: state.ResultError, ExitCode: -1, Failure: "missing directory"}, want: "Could not run"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			row := oneRow("h1")
			result := tt.result
			row.Test = &result

			body := renderPanel(t, i18n.En, viewWith(row))

			assert.Contains(t, body, `data-hook-test="`+string(tt.result.Status)+`"`)
			assert.Contains(t, body, tt.want)
			if tt.wantExit != "" {
				assert.Contains(t, body, tt.wantExit)
			} else {
				assert.NotContains(t, body, "exit code")
			}
		})
	}
}

// The timeout field says the limits it enforces: whole seconds, at most the domain maximum of 86400.
func TestEditorShowsTheTimeoutLimit(t *testing.T) {
	t.Parallel()
	view := viewWith()
	view.Editor = &state.HookForm{Event: "task_start", Phase: "pre", Kind: "script", Timeout: "600"}

	body := renderPanel(t, i18n.En, view)

	assert.Contains(t, body, "Whole seconds, at most 86400.")
	assert.Contains(t, body, `min="1"`)
}
