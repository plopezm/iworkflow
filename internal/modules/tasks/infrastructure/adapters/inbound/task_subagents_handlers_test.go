package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// subagentsReaderFake answers the subagents of a task and counts the reads.
type subagentsReaderFake struct {
	list  []models.SubagentTask
	err   error
	calls int
}

func (f *subagentsReaderFake) Execute(_ context.Context, _ models.TaskID) ([]models.SubagentTask, error) {
	f.calls++
	return f.list, f.err
}

// getSubagentsPanel reads the subagents panel of a task in one language and returns the answer.
func getSubagentsPanel(t *testing.T, reader *subagentsReaderFake, locale i18n.Locale, target string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewDetailHandlers(DetailUseCases{Subagents: reader}, time.Now)
	router := chi.NewRouter()
	h.Mount(router)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = req.WithContext(inLanguage(locale))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// subagentsOfTheTask is a running subagent with a label and some activity, and a finished one without a label.
func subagentsOfTheTask() []models.SubagentTask {
	label := "tests"
	prompt := "revisa los tests"
	return []models.SubagentTask{
		{
			ID:        "s1",
			Agent:     "claude",
			Label:     &label,
			Prompt:    &prompt,
			Status:    models.SubagentRunning,
			Turns:     3,
			ToolCalls: 2,
			Thread:    []models.SubagentThreadItem{models.SubagentText{Text: "mirando auth"}},
		},
		{ID: "s2", Agent: "pi", Status: models.SubagentCompleted},
	}
}

func TestSubagentsPanelListsEachSubagentWithItsStatusAndActivity(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	reader := &subagentsReaderFake{list: subagentsOfTheTask()}

	rec := getSubagentsPanel(t, reader, i18n.Es, "/tasks/"+id+"/subagents?scope=abc123")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, reader.calls)
	assert.Contains(t, rec.Body.String(), `id="task-subagents-abc123"`)
	assert.Contains(t, rec.Body.String(), "claude · tests", "the agent and its label")
	assert.Contains(t, rec.Body.String(), "Ejecutando", "the status of the running subagent")
	assert.Contains(t, rec.Body.String(), "3 turnos · 2 herramientas")
	assert.Contains(t, rec.Body.String(), "Completado", "the status of the finished subagent")
}

// The panel refreshes itself while it stays open, as the transcript does, and its rows keep the scope.
func TestSubagentsPanelRefreshesWhileItStaysOpenAndKeepsTheScope(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	reader := &subagentsReaderFake{list: subagentsOfTheTask()}

	rec := getSubagentsPanel(t, reader, i18n.En, "/tasks/"+id+"/subagents?scope=abc123")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `hx-get="/tasks/`+id+`/subagents?scope=abc123"`)
	assert.Contains(t, rec.Body.String(), `hx-trigger="sse:message from:body delay:300ms"`)
	assert.Contains(t, rec.Body.String(), `hx-get="/tasks/`+id+`/subagents?scope=abc123&amp;selected=s1"`, "each row opens its detail")
}

func TestSubagentsPanelShowsTheSelectedSubagentWithItsPromptAndThread(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	reader := &subagentsReaderFake{list: subagentsOfTheTask()}

	selected := getSubagentsPanel(t, reader, i18n.En, "/tasks/"+id+"/subagents?scope=abc123&selected=s1")
	require.Equal(t, http.StatusOK, selected.Code)
	assert.Contains(t, selected.Body.String(), "revisa los tests", "the prompt of the selected subagent")
	assert.Contains(t, selected.Body.String(), "mirando auth", "its thread")

	vanished := getSubagentsPanel(t, reader, i18n.En, "/tasks/"+id+"/subagents?scope=abc123&selected=gone")
	require.Equal(t, http.StatusOK, vanished.Code)
	assert.NotContains(t, vanished.Body.String(), "revisa los tests", "a subagent that left the list shows no detail")
}

func TestSubagentsPanelSaysSoWhenTheAgentLaunchedNone(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()

	rec := getSubagentsPanel(t, &subagentsReaderFake{}, i18n.Es, "/tasks/"+id+"/subagents?scope=abc123")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Este agente no ha lanzado subagentes.")
}

func TestSubagentsOfAnUnknownTaskShowTheErrorInPlace(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	reader := &subagentsReaderFake{err: errors.New("la tarea no existe")}

	rec := getSubagentsPanel(t, reader, i18n.Es, "/tasks/"+id+"/subagents?scope=abc123")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "la tarea no existe")
	assert.Contains(t, rec.Body.String(), "text-error")
}

func TestSubagentsOfAnInvalidIdentifierDoNotReadTheRuntime(t *testing.T) {
	t.Parallel()
	reader := &subagentsReaderFake{}

	rec := getSubagentsPanel(t, reader, i18n.En, "/tasks/no-es-un-uuid/subagents")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, reader.calls)
	assert.Contains(t, rec.Body.String(), "text-error")
}

// A tool call that is still running and failed shows both badges, with its arguments and its output.
func TestSubagentsPanelShowsARunningToolCallAndItsError(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	args := `{"path":"auth.rs"}`
	output := "permission denied"
	reader := &subagentsReaderFake{list: []models.SubagentTask{{
		ID:     "s1",
		Agent:  "claude",
		Status: models.SubagentFailed,
		Thread: []models.SubagentThreadItem{models.SubagentTool{Name: "read", Args: &args, Running: true, IsError: true, Output: &output}},
	}}}

	rec := getSubagentsPanel(t, reader, i18n.En, "/tasks/"+id+"/subagents?scope=abc123&selected=s1")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `badge-info">running<`, "the call is still running")
	assert.Contains(t, rec.Body.String(), `badge-error">error<`, "the call failed")
	assert.Contains(t, rec.Body.String(), `{&#34;path&#34;:&#34;auth.rs&#34;}`, "the arguments of the call")
	assert.Contains(t, rec.Body.String(), "permission denied", "the output of the call")
}
