package tasks_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// recordHistory persists the messages of a task, in order, and returns the sequence number of the last one.
func recordHistory(t *testing.T, repo *Repo, id models.TaskID, messages ...models.AgentEvent) int64 {
	t.Helper()
	var last int64
	for _, message := range messages {
		event, err := repo.Tasks.AppendEvent(bg(), id, models.AgentEventKind{Event: message})
		require.NoError(t, err)
		require.NotNil(t, event.Seq)
		last = *event.Seq
	}
	return last
}

// The refresh of the transcript answers the body only: the persisted messages, not the page around them.
func TestTranscriptEndpointAnswersTheHistoryOfTheTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "transcripcion", models.StatusIdle)
	insertTask(t, repo, task)
	recordHistory(t, repo, task.ID,
		models.UserMessage{Text: "hola"},
		models.AssistantMessage{Text: "eco: jobs-wait"},
	)
	_, router := boardRouter(t, repo)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tasks/"+task.ID.String()+"/transcript", nil))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "hola")
	assert.Contains(t, body, "eco: jobs-wait")
	assert.NotContains(t, body, "<html", "the refresh answers the body, not the page")
	assert.NotContains(t, body, "sse-connect", "the stream belongs to the page, not to the body")
}

func TestTranscriptEndpointShowsAMalformedIdentifierAsAnError(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	_, router := boardRouter(t, repo)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tasks/no-es-un-id/transcript", nil))

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "text-error")
	assert.NotContains(t, response.Body.String(), "msg-user")
}

// The detail page opens no stream of its own (each one would hold a browser connection for good): its transcript
// listens to the stream of the page, for the activity of its task only. A backlog task has no transcript yet.
func TestDetailPageSharesThePageStreamInsteadOfOpeningOne(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "con-historial", models.StatusIdle)
	insertTask(t, repo, task)
	recordHistory(t, repo, task.ID, models.UserMessage{Text: "primero"}, models.AssistantMessage{Text: "segundo"})
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.Es, task.ID.String())

	assert.NotContains(t, body, "sse-connect")
	assert.Contains(t, body, `hx-get="/tasks/`+task.ID.String()+`/transcript"`)
	assert.Contains(t, body, `hx-trigger="sse:activity[event.detail.data==&#39;`+task.ID.String()+`&#39;]`)
	assert.Contains(t, body, "primero")
	assert.Contains(t, body, "segundo")
}

func TestDetailPageWithoutHistoryShowsTheEmptyTranscript(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "sin-historial", models.StatusIdle)
	insertTask(t, repo, task)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.Es, task.ID.String())

	assert.NotContains(t, body, "sse-connect")
	assert.Contains(t, body, "Sin actividad todavía.")
}
