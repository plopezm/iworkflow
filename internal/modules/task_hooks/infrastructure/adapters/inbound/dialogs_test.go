package inbound_test

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
)

// The library holds the list only: no assignment, no test form and no run order are embedded under the rows.
func TestLibraryHoldsOnlyTheList(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("lint", "make")
	require.Equal(t, http.StatusOK, f.assign(hookID, "ws-1", "task_start", "pre").Code)

	body := f.send(http.MethodGet, "/hooks/library", nil).Body.String()

	assert.Contains(t, body, `data-hook="`+hookID+`"`)
	assert.NotContains(t, body, "data-assignment")
	assert.NotContains(t, body, "data-hook-test-form")
	assert.NotContains(t, body, "data-hooks-group")
	assert.NotContains(t, body, "<dialog")
}

// Opening a hook shows its assignments in place of the list, with the back control and the two actions.
func TestOpeningAHookShowsItsAssignmentsView(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("lint", "make")
	require.Equal(t, http.StatusOK, f.assign(hookID, "ws-1", "task_start", "pre").Code)

	body := f.send(http.MethodGet, "/hooks/"+hookID, nil).Body.String()

	assert.Contains(t, body, `data-hook-assignments="`+hookID+`"`)
	assert.Contains(t, body, `data-assignment="`)
	assert.Contains(t, body, `hx-get="/hooks/`+hookID+`/assignments/new"`)
	assert.Contains(t, body, `hx-get="/hooks/`+hookID+`/test/new"`)
	assert.Contains(t, body, `hx-get="/hooks/library"`)
	assert.NotContains(t, body, `data-hook="`+hookID+`"`, "the list is replaced by the view")
	assert.NotContains(t, body, "<dialog")
}

// The add-assignment form opens as a dialog over the view; a saved assignment closes it.
func TestAssignDialogOpensAndClosesOnSave(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("lint", "make")

	open := f.send(http.MethodGet, "/hooks/"+hookID+"/assignments/new", nil)

	require.Equal(t, http.StatusOK, open.Code)
	assert.Contains(t, open.Body.String(), "<dialog")
	assert.Contains(t, open.Body.String(), "data-assignment-form")
	assert.Contains(t, open.Body.String(), `data-dismiss-path="/hooks/`+hookID+`"`)

	saved := f.assign(hookID, "ws-1", "task_start", "pre")

	require.Equal(t, http.StatusOK, saved.Code)
	assert.NotContains(t, saved.Body.String(), "<dialog", "the dialog closes when the assignment is saved")
	assert.Contains(t, saved.Body.String(), "Assignment added.")
	assert.Contains(t, saved.Body.String(), `data-assignment="`)
}

// A refused assignment keeps the dialog open with the reason, and the view behind it is the hook's.
func TestAssignDialogStaysOpenOnARefusal(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("lint", "make")
	require.Equal(t, http.StatusOK, f.assign(hookID, "ws-1", "task_start", "pre").Code)

	refused := f.assign(hookID, "ws-1", "task_start", "pre")

	require.Equal(t, http.StatusOK, refused.Code)
	body := refused.Body.String()
	assert.Contains(t, body, "<dialog", "the dialog stays open")
	assert.Contains(t, body, "data-assignment-form")
	assert.Contains(t, body, "The action failed:")
	assert.Contains(t, body, `data-hook-assignments="`+hookID+`"`)
}

// The test is a dialog that keeps its run on screen until it is closed.
func TestTestDialogKeepsTheRunOnScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("missing", "/does/not/exist/hook")

	open := f.send(http.MethodGet, "/hooks/"+hookID+"/test/new", nil)

	require.Equal(t, http.StatusOK, open.Code)
	assert.Contains(t, open.Body.String(), "<dialog")
	assert.Contains(t, open.Body.String(), "data-hook-test-form")

	run := f.send(http.MethodPost, "/hooks/"+hookID+"/test", url.Values{
		dto.FieldProject: {"ws-1"}, dto.FieldEvent: {"task_start"}, dto.FieldPhase: {"pre"},
	})

	require.Equal(t, http.StatusOK, run.Code)
	assert.Contains(t, run.Body.String(), "<dialog", "the modal stays open to show the run")
	assert.Contains(t, run.Body.String(), `data-hook-test="error"`)
}

// The editor is a dialog: a new hook opens it, and a saved hook closes it.
func TestEditorOpensAsADialogAndClosesOnSave(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	open := f.send(http.MethodGet, "/hooks/editor", nil)

	require.Equal(t, http.StatusOK, open.Code)
	assert.Contains(t, open.Body.String(), "<dialog")
	assert.Contains(t, open.Body.String(), `data-dismiss-path="/hooks/library"`)

	saved := f.send(http.MethodPost, "/hooks", url.Values{
		dto.FieldName: {"lint"}, dto.FieldKind: {"script"}, dto.FieldTimeout: {"30"}, dto.FieldCommand: {"make"},
	})

	require.Equal(t, http.StatusOK, saved.Code)
	assert.NotContains(t, saved.Body.String(), "<dialog")
	assert.Contains(t, saved.Body.String(), "Hook created.")
}

// The run order is a tab of its own: the group answers alone, and the moves submit the whole order.
func TestOrderTabServesTheRunOrderAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("lint", "make")
	require.Equal(t, http.StatusOK, f.assign(hookID, "ws-1", "task_start", "pre").Code)

	rec := f.send(http.MethodGet, "/hooks/group?project=ws-1&event=task_start&phase=pre", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `data-hooks-group`)
	assert.Contains(t, body, `data-group-row=`)
	assert.NotContains(t, body, "data-hooks-library")
	assert.NotContains(t, body, "<details")
	assert.Regexp(t, regexp.MustCompile(`aria-pressed="true"[^>]*hx-get="/hooks/group"`), body, "the run order tab is the open one")
}

// The order tab is shown after a move, so the answer of a move keeps the user on the run order.
func TestMoveAnswersTheRunOrderTab(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	first := f.saveScript("first", "make")
	second := f.saveScript("second", "make")
	require.Equal(t, http.StatusOK, f.assign(first, "ws-1", "task_start", "pre").Code)
	require.Equal(t, http.StatusOK, f.assign(second, "ws-1", "task_start", "pre").Code)
	order := f.assignmentID(first, "ws-1", "task_start", "pre") + "," + f.assignmentID(second, "ws-1", "task_start", "pre")

	rec := f.send(http.MethodPost, "/hooks/order", url.Values{
		dto.FieldProject: {"ws-1"}, dto.FieldEvent: {"task_start"}, dto.FieldPhase: {"pre"}, dto.FieldOrder: {order},
	})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Order saved.")
	assert.Contains(t, rec.Body.String(), `data-hooks-group`)
	assert.Regexp(t, regexp.MustCompile(`aria-pressed="true"[^>]*hx-get="/hooks/group"`), rec.Body.String())
}

// A dialog for a hook that does not exist opens no dialog: its refusal stays on the screen, where it can be seen.
func TestDialogOfAnUnknownHookShowsTheRefusalOnScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	for _, path := range []string{"/hooks/nope/test/new", "/hooks/nope/assignments/new"} {
		rec := f.send(http.MethodGet, path, nil)

		require.Equal(t, http.StatusOK, rec.Code, path)
		assert.NotContains(t, rec.Body.String(), "<dialog", path)
		assert.Contains(t, rec.Body.String(), "The action failed:", path)
	}
}
