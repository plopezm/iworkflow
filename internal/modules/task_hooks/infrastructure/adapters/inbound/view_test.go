package inbound

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// newHook builds a valid library hook for the view tests.
func newHook(t *testing.T, id string, name string) models.Hook {
	t.Helper()
	hook, err := models.NewHook(models.HookParams{ID: id, Name: name, Kind: models.KindScript, Timeout: time.Minute,
		Script: models.ScriptSpec{Command: "make"}})
	require.NoError(t, err)
	return hook
}

// newAssignment attaches a hook to a project, event and moment.
func newAssignment(t *testing.T, id string, hookID string, project string, event models.Event, phase models.Phase, position int) models.Assignment {
	t.Helper()
	assignment, err := models.NewAssignment(models.AssignmentParams{
		ID: id, HookID: hookID, WorkspaceID: project, Event: event, Phase: phase,
		Enabled: true, OnFailure: models.FailureBlock, Position: position,
	})
	require.NoError(t, err)
	return assignment
}

// The group rows are the assignments of one project, event and moment, in position order; the moves carry the whole
// order with the row swapped, and an end of the group has no move in its direction.
func TestGroupRowsSwapTheWholeOrderOfTheGroup(t *testing.T) {
	t.Parallel()
	hookA, hookB, hookC := newHook(t, "h-a", "alpha"), newHook(t, "h-b", "beta"), newHook(t, "h-c", "gamma")
	assigned := []models.AssignedHook{
		{Hook: hookB, Assignment: newAssignment(t, "a2", "h-b", "ws-1", models.EventTaskStart, models.PhasePre, 2)},
		{Hook: hookC, Assignment: newAssignment(t, "a3", "h-c", "ws-1", models.EventTaskStart, models.PhasePre, 3)},
		{Hook: hookA, Assignment: newAssignment(t, "a1", "h-a", "ws-1", models.EventTaskStart, models.PhasePre, 1)},
		{Hook: hookA, Assignment: newAssignment(t, "other", "h-a", "ws-1", models.EventTaskFinish, models.PhasePre, 1)},
	}

	rows := groupRows(assigned, dto.Group{Project: "ws-1", Event: models.EventTaskStart, Phase: models.PhasePre})

	require.Len(t, rows, 3, "the other event is not in the group")
	assert.Equal(t, []string{"a1", "a2", "a3"}, []string{rows[0].ID, rows[1].ID, rows[2].ID})
	assert.Equal(t, "", rows[0].UpOrder, "the first row cannot move up")
	assert.Equal(t, "a2,a1,a3", rows[0].DownOrder)
	assert.Equal(t, "a2,a1,a3", rows[1].UpOrder)
	assert.Equal(t, "a1,a3,a2", rows[1].DownOrder)
	assert.Equal(t, "a1,a3,a2", rows[2].UpOrder)
	assert.Equal(t, "", rows[2].DownOrder, "the last row cannot move down")
	assert.Equal(t, "alpha", rows[0].HookName)
}

// The last error of a hook links to the history of that hook, and shows nothing when it never failed.
func TestLastErrorLinksToTheHookHistory(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC)
	failure := &models.HookRun{ID: "r1", HookID: "h-1", Status: models.RunTimedOut, StartedAt: started}

	last := lastError(failure)

	require.NotNil(t, last)
	assert.Equal(t, "timeout", last.Status)
	assert.Equal(t, "2026-11-07 09:30", last.StartedAt)
	assert.Equal(t, "/hooks/history?applied=1&failed=1&hook=h-1&project=", last.HistoryPath)
	assert.Nil(t, lastError(nil))
}

// Only a script run has an exit code: an agent run, a skipped one and a running one show none.
func TestRunRowsShowAnExitCodeOnlyForScriptRuns(t *testing.T) {
	t.Parallel()
	runs := []models.HookRun{
		{ID: "script", Status: models.RunFailed, ExitCode: 2, WorkspaceID: "ws-1", HookName: "lint"},
		{ID: "agent", Status: models.RunFailed, ExitCode: -1, WorkspaceID: "ws-1", HookName: "review"},
		{ID: "skipped", Status: models.RunSkipped, ExitCode: 0, WorkspaceID: "ws-1", HookName: "lint"},
		{ID: "gone", Status: models.RunOK, ExitCode: 0, WorkspaceID: "ws-removed", HookName: "lint"},
	}

	rows := runRows(runs, map[string]string{"ws-1": "alpha"})

	assert.True(t, rows[0].HasExitCode)
	assert.Equal(t, 2, rows[0].ExitCode)
	assert.False(t, rows[1].HasExitCode)
	assert.False(t, rows[2].HasExitCode)
	assert.Equal(t, "alpha", rows[0].ProjectName)
	assert.Equal(t, "ws-removed", rows[3].ProjectName, "a project that was removed keeps its id on the record")
}

// The assignments of a hook are sorted by project, event, moment and position, so the list reads the same every time.
func TestHookDetailSortsTheAssignments(t *testing.T) {
	t.Parallel()
	hook := newHook(t, "h-1", "lint")
	item := application.HookListItem{Hook: hook, Assignments: []models.Assignment{
		newAssignment(t, "a-z", "h-1", "ws-z", models.EventTaskStart, models.PhasePre, 1),
		newAssignment(t, "a-b-post", "h-1", "ws-b", models.EventTaskStart, models.PhasePost, 1),
		newAssignment(t, "a-b-pre", "h-1", "ws-b", models.EventTaskStart, models.PhasePre, 1),
	}}
	names := map[string]string{"ws-b": "beta", "ws-z": "zeta"}

	detail := hookDetail(item, names, state.AssignmentForm{}, state.AssignmentForm{}, nil)

	require.Len(t, detail.Assignments, 3)
	assert.Equal(t, []string{"a-b-pre", "a-b-post", "a-z"}, []string{detail.Assignments[0].Form.ID, detail.Assignments[1].Form.ID, detail.Assignments[2].Form.ID})
	assert.Equal(t, "beta", detail.Assignments[0].ProjectName)
	assert.Equal(t, "lint", detail.Name)
}

// A stored hook is edited with its values, and a rejected form keeps the values as the user typed them.
func TestHookFormKeepsTheValuesAsTyped(t *testing.T) {
	t.Parallel()
	values := url.Values{
		dto.FieldID: {"h-1"}, dto.FieldName: {"lint"}, dto.FieldKind: {"script"}, dto.FieldTimeout: {"soon"},
		dto.FieldCommand: {"make"}, dto.FieldUseShell: {"on"}, dto.FieldBrowseProject: {"ws-2"},
	}

	form := hookFormFromValues(values)

	assert.Equal(t, state.HookForm{
		ID: "h-1", Name: "lint", Kind: "script", Timeout: "soon", Command: "make", UseShell: true, BrowseProject: "ws-2",
	}, form)
}

// A stored hook opens its editor with its values; an agent hook has no script fields.
func TestHookFormFromAStoredHook(t *testing.T) {
	t.Parallel()
	hook, err := models.NewHook(models.HookParams{
		ID: "h-1", Name: "review", Kind: models.KindAgent, Timeout: 2 * time.Minute,
		Agent: models.AgentSpec{Prompt: "check the diff", Model: "anthropic/sonnet"},
	})
	require.NoError(t, err)

	form := hookFormFromHook(hook)

	assert.Equal(t, "review", form.Name)
	assert.Equal(t, "agent", form.Kind)
	assert.Equal(t, "120", form.Timeout)
	assert.Equal(t, "check the diff", form.Prompt)
	assert.Equal(t, "anthropic/sonnet", form.Model)
	assert.Empty(t, form.Command)
}
