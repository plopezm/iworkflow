package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

const diffTestPatch = "diff --git a/src/a.rs b/src/a.rs\n--- a/src/a.rs\n+++ b/src/a.rs\n@@ -1 +1 @@\n-a\n+b\n"

// diffTaskFake answers the task the tab reads for its title and its existence.
type diffTaskFake struct {
	task models.Task
	err  error
}

func (f *diffTaskFake) Execute(_ context.Context, _ models.TaskID) (models.Task, error) {
	return f.task, f.err
}

// diffReaderFake answers the diff and counts the reads.
type diffReaderFake struct {
	diff  models.TaskDiff
	err   error
	calls int
}

func (f *diffReaderFake) Execute(_ context.Context, _ models.TaskID) (models.TaskDiff, error) {
	f.calls++
	return f.diff, f.err
}

// newDiffTaskForTest is a task with a known title and a fresh identifier.
func newDiffTaskForTest(t *testing.T) models.Task {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return models.Task{ID: id, Title: "Fix login"}
}

func newDiffHandlersForTest(get *diffTaskFake, diff *diffReaderFake) *DiffHandlers {
	return NewDiffHandlers(DiffUseCases{Get: get, Diff: diff})
}

// inLanguage is a request context in one language, as the shell sets it.
func inLanguage(locale i18n.Locale) context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
}

func renderDiffTab(t *testing.T, h *DiffHandlers, locale i18n.Locale, rawID string) string {
	t.Helper()
	ctx := inLanguage(locale)
	routes := h.Tabs()
	require.Len(t, routes, 1)
	var out strings.Builder
	require.NoError(t, routes[0].Render(ctx, dock.Params{"id": rawID}).Render(ctx, &out))
	return out.String()
}

// diffViewRequest asks the view of a task for a query, as htmx does from a control of the view.
func diffViewRequest(t *testing.T, h *DiffHandlers, locale i18n.Locale, rawID string, query string) (int, string) {
	t.Helper()
	router := chi.NewRouter()
	h.Mount(router)
	request := httptest.NewRequest(http.MethodGet, "/tasks/"+rawID+"/diff/view?"+query, nil)
	request.Header.Set("HX-Request", "true")
	request = request.WithContext(inLanguage(locale))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}

func TestDiffTabIsTheDockRouteOfTheTaskDiff(t *testing.T) {
	t.Parallel()
	h := newDiffHandlersForTest(&diffTaskFake{}, &diffReaderFake{})

	routes := h.Tabs()

	require.Len(t, routes, 1)
	assert.Equal(t, TaskDiffRouteName, routes[0].Name)
	assert.Equal(t, "/tasks/{id}/diff", routes[0].Pattern)
	assert.Equal(t, "±", routes[0].Icon)
}

func TestDiffTabTitleNamesTheTask(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{})
	route := h.Tabs()[0]

	title := route.Title(inLanguage(i18n.En), dock.Params{"id": task.ID.String()})

	assert.Equal(t, "Diff · Fix login", title)
}

func TestDiffTabTitleFallsBackWhenTheTaskIsUnknown(t *testing.T) {
	t.Parallel()
	h := newDiffHandlersForTest(&diffTaskFake{err: werrors.NotFound("x")}, &diffReaderFake{})

	title := h.Tabs()[0].Title(inLanguage(i18n.En), dock.Params{"id": newDiffTaskForTest(t).ID.String()})

	assert.Equal(t, "Diff", title)
}

func TestDiffTabShowsTheDiffOfTheTask(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	reader := &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch, Commits: []string{"abc1234 fix"}}}
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, reader)

	body := renderDiffTab(t, h, i18n.En, task.ID.String())

	assert.Contains(t, body, "src/a.rs")
	assert.Contains(t, body, ">Split<")
	assert.Contains(t, body, `id="diff-view-`)
	assert.Equal(t, 1, reader.calls)
}

func TestDiffTabSaysSoForATaskThatNoLongerExists(t *testing.T) {
	t.Parallel()
	h := newDiffHandlersForTest(&diffTaskFake{err: werrors.NotFound("x")}, &diffReaderFake{})

	body := renderDiffTab(t, h, i18n.En, newDiffTaskForTest(t).ID.String())

	assert.Contains(t, body, "This task no longer exists.")
}

func TestDiffTabRejectsAnInvalidIdentifierInTheRequestLanguage(t *testing.T) {
	t.Parallel()
	get := &diffTaskFake{}
	h := newDiffHandlersForTest(get, &diffReaderFake{})

	body := renderDiffTab(t, h, i18n.Es, "nope")

	assert.Contains(t, body, msgInvalidTask.Es)
	assert.NotContains(t, body, "no válido: nope")
}

func TestDiffTabSaysThatTheDiffCannotBeReadWithoutTheInternalText(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{err: errors.New("git exploded")})

	body := renderDiffTab(t, h, i18n.Fr, task.ID.String())

	assert.Contains(t, body, msgDiffUnavailable.Fr)
	assert.NotContains(t, body, "git exploded")
}

func TestDiffViewAnswersTheLayoutItWasAsked(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}})

	code, body := diffViewRequest(t, h, i18n.En, task.ID.String(), "view=unified&scope=ab12")

	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `id="diff-view-ab12"`)
	assert.Contains(t, body, `class="diff-sign"`)
	assert.Regexp(t, `diff-segment-active" aria-pressed="true"[^>]*>Unified</button>`, body)
}

func TestDiffViewWithoutTheTreeHasNoSidebar(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}})

	_, body := diffViewRequest(t, h, i18n.En, task.ID.String(), "tree=0")

	assert.NotContains(t, body, `class="diff-sidebar"`)
	assert.Contains(t, body, `class="diff-cards"`)
}

func TestDiffViewDropsAScopeThatIsNotAPageInstance(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}})

	_, body := diffViewRequest(t, h, i18n.En, task.ID.String(), "scope=BAD!")

	assert.Contains(t, body, `id="diff-view"`)
	assert.NotContains(t, body, "BAD!")
}

func TestDiffViewSaysThatTheDiffCannotBeReadInTheRequestLanguage(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{err: errors.New("boom")})

	code, body := diffViewRequest(t, h, i18n.Es, task.ID.String(), "")

	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, msgDiffUnavailable.Es)
	assert.NotContains(t, body, "boom")
}

func TestDiffViewRejectsAnInvalidIdentifier(t *testing.T) {
	t.Parallel()
	reader := &diffReaderFake{}
	h := newDiffHandlersForTest(&diffTaskFake{}, reader)

	_, body := diffViewRequest(t, h, i18n.En, "nope", "")

	assert.Contains(t, body, msgInvalidTask.En)
	assert.Equal(t, 0, reader.calls)
}
