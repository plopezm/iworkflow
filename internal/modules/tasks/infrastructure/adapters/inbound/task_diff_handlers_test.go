package inbound

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
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
	"iworkflow/internal/modules/tasks/presentation/state"
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
	return NewDiffHandlers(DiffUseCases{Get: get, Diff: diff, Send: &fakeSender{}, History: &fakeHistory{}})
}

// explainedHistory is the history of a task whose agent was asked for the explanation and answered it.
func explainedHistory(id models.TaskID, answered bool) []models.TaskEvent {
	events := []models.TaskEvent{{TaskID: id, Kind: models.AgentEventKind{Event: models.UserMessage{Text: state.ExplanationPrompt}}}}
	if answered {
		events = append(events, models.TaskEvent{TaskID: id, Kind: models.AgentEventKind{Event: models.AssistantMessage{Text: "EXPLAINED-BODY"}}})
	}
	return events
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

// diffReportRequest asks a printable report action of the task, as htmx does from a control of the report.
func diffReportRequest(t *testing.T, h *DiffHandlers, locale i18n.Locale, rawID string, query string) (int, string) {
	t.Helper()
	router := chi.NewRouter()
	h.Mount(router)
	request := httptest.NewRequest(http.MethodGet, "/tasks/"+rawID+"/diff/report?"+query, nil)
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

// A view that cannot read the diff answers the region with its controls still in it, so the layout can be changed again
// and the rest of the page is not lost.
func TestDiffViewKeepsTheControlsWhenTheDiffCannotBeRead(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{err: errors.New("git exploded")})

	code, body := diffViewRequest(t, h, i18n.En, task.ID.String(), "scope=ab12&view=unified")

	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `id="diff-view-ab12"`)
	assert.Contains(t, body, `class="diff-toolbar"`)
	assert.Contains(t, body, msgDiffUnavailable.En)
	assert.NotContains(t, body, "git exploded")
}

// A task that no longer exists keeps the region too, so the view that asked is still the one that answers.
func TestDiffViewSaysSoForATaskThatNoLongerExistsInItsRegion(t *testing.T) {
	t.Parallel()
	h := newDiffHandlersForTest(&diffTaskFake{err: werrors.NotFound("x")}, &diffReaderFake{})

	code, body := diffViewRequest(t, h, i18n.En, newDiffTaskForTest(t).ID.String(), "scope=ab12")

	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `id="diff-view-ab12"`)
	assert.Contains(t, body, "This task no longer exists.")
}

// The read error goes to the log with the task it belongs to, so the failure can be followed to its task. The default
// logger is swapped, so this test does not run in parallel.
func TestDiffViewLogsTheTaskWhoseDiffCouldNotBeRead(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{err: errors.New("git exploded")})

	diffViewRequest(t, h, i18n.En, task.ID.String(), "")

	assert.Contains(t, logs.String(), "task="+task.ID.String())
	assert.Contains(t, logs.String(), "git exploded")
}

// The plain report renders the printable content of the diff: the toolbar of the report, the commits, the summary
// and the patch grouped per file, each line with its class.
func TestDiffReportRendersThePlainReport(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch, Commits: []string{"abc1234 fix"}}})

	code, body := diffReportRequest(t, h, i18n.En, task.ID.String(), "action=plain")

	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `class="print-report"`)
	assert.Contains(t, body, `class="print-report-toolbar"`)
	assert.Contains(t, body, `class="print-report-page"`)
	assert.Contains(t, body, task.Title)
	assert.Contains(t, body, `class="print-report-block"`)
	assert.Contains(t, body, `class="diff-add"`)
	assert.Contains(t, body, `class="diff-del"`)
}

// The explained report answers the task with the report that waits for the agent, behind the progress dialog.
func TestDiffReportExplainedWaitsForTheAgentAnswer(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}})

	code, body := diffReportRequest(t, h, i18n.En, task.ID.String(), "action=explained")

	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Generating PDF")
}

// The explained report sends the prompt and keeps asking for the answer: the dialog polls with the length of the
// transcript at the time of the ask, so the answers before it are ignored.
func TestDiffReportExplainedPollsFromTheTranscriptItWasAskedAt(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	sender := &fakeSender{}
	history := &fakeHistory{events: []models.TaskEvent{{TaskID: task.ID, Kind: models.AgentEventKind{Event: models.AssistantMessage{Text: "old"}}}}}
	h := NewDiffHandlers(DiffUseCases{Get: &diffTaskFake{task: task}, Diff: &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}}, Send: sender, History: history})

	_, body := diffReportRequest(t, h, i18n.En, task.ID.String(), "action=explained")

	require.Len(t, sender.calls, 1)
	assert.Equal(t, state.ExplanationPrompt, sender.calls[0].message)
	assert.Contains(t, body, "Generating PDF")
	assert.Contains(t, body, "action=poll")
	assert.Contains(t, body, "since=1")
}

// The poll shows the report with the answer of the agent once it answered, and keeps waiting before that.
func TestDiffReportPollRendersTheAnswerWhenTheAgentAnswered(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	reader := &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}}
	waiting := NewDiffHandlers(DiffUseCases{Get: &diffTaskFake{task: task}, Diff: reader, Send: &fakeSender{}, History: &fakeHistory{events: explainedHistory(task.ID, false)}})
	answered := NewDiffHandlers(DiffUseCases{Get: &diffTaskFake{task: task}, Diff: reader, Send: &fakeSender{}, History: &fakeHistory{events: explainedHistory(task.ID, true)}})

	_, stillWaiting := diffReportRequest(t, waiting, i18n.En, task.ID.String(), "action=poll&since=0")
	_, ready := diffReportRequest(t, answered, i18n.En, task.ID.String(), "action=poll&since=0")

	assert.Contains(t, stillWaiting, "Generating PDF")
	assert.NotContains(t, stillWaiting, `class="print-report"`)
	assert.Contains(t, ready, `class="print-report"`)
	assert.Contains(t, ready, "EXPLAINED-BODY")
	assert.NotContains(t, ready, "Generating PDF")
}

// A send that fails is shown with its error and a way out, not as an empty region.
func TestDiffReportExplainedShowsTheSendFailure(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	sender := &fakeSender{err: errors.New("task is not accepting messages")}
	h := NewDiffHandlers(DiffUseCases{Get: &diffTaskFake{task: task}, Diff: &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}}, Send: sender, History: &fakeHistory{}})

	_, body := diffReportRequest(t, h, i18n.En, task.ID.String(), "action=explained")

	assert.Contains(t, body, msgExplanationNotSent.En)
	assert.NotContains(t, body, "task is not accepting messages")
	assert.Contains(t, body, "action=close")
	assert.NotContains(t, body, "Generating PDF")
	assert.NotContains(t, body, "action=poll")
}

// The close of the report stops the report and answers the diff view again: the summary and the file cards of
// the task diff come back, without the report content.
func TestDiffReportCloseStopsWaiting(t *testing.T) {
	t.Parallel()
	task := newDiffTaskForTest(t)
	reader := &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch, Commits: []string{"abc1234 fix"}}}
	h := newDiffHandlersForTest(&diffTaskFake{task: task}, reader)

	code, body := diffReportRequest(t, h, i18n.En, task.ID.String(), "action=close")

	assert.Equal(t, http.StatusOK, code)
	assert.NotContains(t, body, `class="print-report"`)
	assert.Contains(t, body, `class="diff-summary"`)
	assert.Contains(t, body, `class="diff-cards"`)
	assert.Contains(t, body, "src/a.rs")
	assert.Equal(t, 1, reader.calls)
}

// The report answers an action that is not one of the reports of the task.
func TestDiffReportRejectsAnUnknownAction(t *testing.T) {
	t.Parallel()
	h := newDiffHandlersForTest(&diffTaskFake{}, &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}})

	code, body := diffReportRequest(t, h, i18n.En, newDiffTaskForTest(t).ID.String(), "action=nope")

	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, msgInvalidReportAction.En)
}

func TestDiffViewRejectsAnInvalidIdentifier(t *testing.T) {
	t.Parallel()
	reader := &diffReaderFake{}
	h := newDiffHandlersForTest(&diffTaskFake{}, reader)

	_, body := diffViewRequest(t, h, i18n.En, "nope", "")

	assert.Contains(t, body, msgInvalidTask.En)
	assert.Equal(t, 0, reader.calls)
}

// The files of a diff can be edited only while the task has its worktree, so the cards offer the editor then.
func TestDiffTabOffersTheEditorOnlyWhileTheTaskHasItsWorktree(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		status  models.TaskStatus
		offered bool
	}{
		{name: "running task keeps its worktree", status: models.StatusRunning, offered: true},
		{name: "merged task has no worktree", status: models.StatusMerged, offered: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			task := newDiffTaskForTest(t)
			task.Status = tc.status
			h := newDiffHandlersForTest(&diffTaskFake{task: task}, &diffReaderFake{diff: models.TaskDiff{Patch: diffTestPatch}})

			html := renderDiffTab(t, h, i18n.En, task.ID.String())

			assert.Equal(t, tc.offered, strings.Contains(html, `hx-get="/tasks/`+task.ID.String()+`/file?`))
		})
	}
}
