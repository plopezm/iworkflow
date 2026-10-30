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

// mergeFake answers the outcome of the merge and counts the calls.
type mergeFake struct {
	outcome models.MergeOutcome
	err     error
	calls   int
}

func (f *mergeFake) Execute(_ context.Context, _ models.TaskID) (models.MergeOutcome, error) {
	f.calls++
	return f.outcome, f.err
}

// detailHistoryFake is an empty history: the merge answer does not depend on the transcript.
type detailHistoryFake struct{}

func (detailHistoryFake) Execute(context.Context, models.TaskID, *int64) ([]models.TaskEvent, error) {
	return nil, nil
}

// detailAttachmentsFake is an empty list of images.
type detailAttachmentsFake struct{}

func (detailAttachmentsFake) Execute(context.Context, models.TaskID) ([]models.StoredAttachment, error) {
	return nil, nil
}

// postMerge posts the merge of a task in one language and returns the answer.
func postMerge(t *testing.T, merger *mergeFake, locale i18n.Locale, id string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewDetailHandlers(DetailUseCases{
		Get:             &diffTaskFake{task: newDiffTaskForTest(t)},
		Events:          detailHistoryFake{},
		ListAttachments: detailAttachmentsFake{},
		Merge:           merger,
	}, time.Now)
	router := chi.NewRouter()
	h.Mount(router)
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+id+"/merge?scope=abc123", nil)
	req = req.WithContext(inLanguage(locale))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestMergeOfAnIntegratedTaskAnswersThePageWithTheNoticeInSpanish(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	merger := &mergeFake{outcome: models.MergedOutcome()}

	rec := postMerge(t, merger, i18n.Es, id)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, merger.calls)
	assert.Equal(t, boardRefreshEvent, rec.Header().Get("HX-Trigger"), "the board moves the task to done")
	assert.Contains(t, rec.Body.String(), `id="task-detail-abc123"`, "the answer is the page of this instance")
	assert.Contains(t, rec.Body.String(), "Integrada en la rama base.")
}

func TestMergeWithConflictsNamesTheFilesThatClash(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	merger := &mergeFake{outcome: models.ConflictOutcome([]string{"a.go", "b.go"})}

	rec := postMerge(t, merger, i18n.Es, id)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Conflictos en a.go, b.go.")
	assert.NotContains(t, rec.Body.String(), "Integrada en la rama base.")
}

func TestMergeFailureShowsTheErrorInPlaceAndKeepsThePage(t *testing.T) {
	t.Parallel()
	id := newDiffTaskForTest(t).ID.String()
	merger := &mergeFake{err: errors.New("la tarea no se puede integrar")}

	rec := postMerge(t, merger, i18n.Es, id)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "la tarea no se puede integrar")
	assert.Contains(t, rec.Body.String(), "text-error")
	assert.Contains(t, rec.Body.String(), `id="task-detail-abc123"`)
}

func TestMergeOfAnInvalidIdentifierDoesNotRunTheMerge(t *testing.T) {
	t.Parallel()
	merger := &mergeFake{}

	rec := postMerge(t, merger, i18n.En, "no-es-un-uuid")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, merger.calls)
	assert.Contains(t, rec.Body.String(), "text-error")
}
