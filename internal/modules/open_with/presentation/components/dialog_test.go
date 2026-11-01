package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/open_with/presentation/components"
	"iworkflow/internal/modules/open_with/presentation/state"
)

func render(t *testing.T, locale i18n.Locale, view state.DialogView) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale})
	var out bytes.Buffer
	require.NoError(t, components.Dialog(view).Render(ctx, &out))
	return out.String()
}

func TestDialogWithoutRowsShowsTheEmptyMessageAndTheForm(t *testing.T) {
	body := render(t, i18n.En, state.DialogView{TaskID: "t1", Scope: "s"})

	require.Contains(t, body, "No applications configured yet")
	require.Contains(t, body, `id="task-open-with-s"`)
	require.Contains(t, body, `hx-post="/open-with/tasks/t1?scope=s"`)
	require.Contains(t, body, "Browse…")
	require.NotContains(t, body, "data-open-with-list")
	require.NotContains(t, body, "data-open-with-picker")
}

func TestDialogListsRowsWithTheirActionsAndSummary(t *testing.T) {
	body := render(t, i18n.Es, state.DialogView{TaskID: "t1", Rows: []state.Row{{ID: "a1", Name: "Editor", Path: "/usr/bin/code", Args: 2, EnvVars: 1}}})

	require.Contains(t, body, `data-launcher="a1"`)
	require.Contains(t, body, "/usr/bin/code")
	require.Contains(t, body, "2 parámetros · 1 entorno")
	require.Contains(t, body, `hx-post="/open-with/tasks/t1/launchers/a1/launch"`)
	require.Contains(t, body, `hx-post="/open-with/tasks/t1/launchers/a1/delete"`)
	require.Contains(t, body, "Abrir con")
}

func TestDialogShowsTheOutcomeAndKeepsTheTypedValues(t *testing.T) {
	body := render(t, i18n.En, state.DialogView{
		TaskID: "t1", Started: "Editor", Failure: "the path is required",
		Entered: state.Entered{Name: "Mine", Path: "code", Args: "--x", Env: "A=1"},
	})

	require.Contains(t, body, "Started: Editor")
	require.Contains(t, body, "the path is required")
	require.Contains(t, body, `value="Mine"`)
	require.Contains(t, body, `value="code"`)
}

func TestDialogEscapesUserText(t *testing.T) {
	body := render(t, i18n.En, state.DialogView{TaskID: "t1", Rows: []state.Row{{ID: "a1", Name: "<script>x</script>", Path: "/p"}}})

	require.NotContains(t, body, "<script>x</script>")
	require.Contains(t, body, "&lt;script&gt;")
}

func TestPickerShowsFoldersAndFilesWithTheirRequests(t *testing.T) {
	body := render(t, i18n.En, state.DialogView{TaskID: "t1", Picker: &state.Picker{
		Dir: "/opt", Parent: "/", Truncated: true,
		Entries: []state.PickerEntry{{Name: "tools", Path: "/opt/tools", IsDir: true}, {Name: "app", Path: "/opt/app"}},
	}})

	require.Contains(t, body, "data-open-with-picker")
	require.Contains(t, body, "/opt")
	require.Contains(t, body, `/open-with/tasks/t1/browse`)
	require.Contains(t, body, `{&#34;dir&#34;:&#34;/opt/tools&#34;,&#34;filter&#34;:&#34;&#34;}`)
	require.Contains(t, body, `{&#34;file&#34;:&#34;/opt/app&#34;}`)
	require.Contains(t, body, "Showing the first entries only")
}

func TestPickerDisablesUpAtTheRootAndShowsEmptyAndErrors(t *testing.T) {
	empty := render(t, i18n.En, state.DialogView{TaskID: "t1", Picker: &state.Picker{Dir: "/"}})
	failed := render(t, i18n.En, state.DialogView{TaskID: "t1", Picker: &state.Picker{Dir: "/x", Error: "could not read the folder"}})

	require.Contains(t, empty, "Nothing to show here.")
	require.Contains(t, empty, "disabled")
	require.Contains(t, failed, "could not read the folder")
	require.NotContains(t, failed, "Nothing to show here.")
}

// The filter request must not carry a fixed filter value: it would override what the user types.
func TestPickerFilterRequestSendsOnlyTheFolder(t *testing.T) {
	body := render(t, i18n.En, state.DialogView{TaskID: "t1", Picker: &state.Picker{Dir: "/opt", Filter: "ab"}})

	require.Contains(t, body, `hx-vals="{&#34;dir&#34;:&#34;/opt&#34;}"`)
	require.Contains(t, body, `value="ab"`)
}
