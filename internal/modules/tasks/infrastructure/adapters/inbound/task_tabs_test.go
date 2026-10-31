package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// tabsTaskFake answers the task of the detail page.
type tabsTaskFake struct {
	task models.Task
	err  error
}

func (f *tabsTaskFake) Execute(_ context.Context, _ models.TaskID) (models.Task, error) {
	return f.task, f.err
}

// tabsHistoryFake has no events, so the transcript is empty.
type tabsHistoryFake struct{}

func (tabsHistoryFake) Execute(_ context.Context, _ models.TaskID, _ *int64) ([]models.TaskEvent, error) {
	return nil, nil
}

// tabsAttachmentsFake has no images.
type tabsAttachmentsFake struct{}

func (tabsAttachmentsFake) Execute(_ context.Context, _ models.TaskID) ([]models.StoredAttachment, error) {
	return nil, nil
}

// tabsPanelsFake answers every panel with its name and the scope it was asked for, so the tests read what each tab shows.
type tabsPanelsFake struct{}

func (tabsPanelsFake) RenderTree(_ context.Context, scope string) templ.Component {
	return templ.Raw("tree of " + scope)
}

func (tabsPanelsFake) TreePanel(_ context.Context, scope string) templ.Component {
	return templ.Raw("files of " + scope)
}

func (tabsPanelsFake) ChangesPanel(_ context.Context, scope string) templ.Component {
	return templ.Raw("changes of " + scope)
}

func (tabsPanelsFake) RenderFile(_ context.Context, scope string, path string) templ.Component {
	return templ.Raw("file " + path + " of " + scope)
}

func (tabsPanelsFake) MergeRequestPanel(_ context.Context, taskID string) templ.Component {
	return templ.Raw("merge-request of task:" + taskID)
}

// newTabsDetailForTest builds the detail handlers over fakes, with the project panels injected as the composition
// root injects them.
func newTabsDetailForTest(task models.Task, getErr error, panels ProjectPanels) *DetailHandlers {
	return NewDetailHandlers(DetailUseCases{
		Get:             &tabsTaskFake{task: task, err: getErr},
		Events:          tabsHistoryFake{},
		ListAttachments: tabsAttachmentsFake{},
		Panels:          panels,
	}, time.Now)
}

// newRunningTaskForTabs is a task with a worktree, so its files and changes exist.
func newRunningTaskForTabs(t *testing.T) models.Task {
	t.Helper()
	task := newDiffTaskForTest(t)
	task.Status = models.StatusRunning
	return task
}

// serveTabs asks one address of the detail page, as htmx does from a tab.
func serveTabs(t *testing.T, h *DetailHandlers, locale i18n.Locale, target string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	h.Mount(router)
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("HX-Request", "true")
	request = request.WithContext(inLanguage(locale))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// renderTabsRoute renders one dock route of the detail handlers with the given params.
func renderTabsRoute(t *testing.T, h *DetailHandlers, name string, params dock.Params) (string, string) {
	t.Helper()
	ctx := inLanguage(i18n.En)
	for _, route := range h.Tabs() {
		if route.Name != name {
			continue
		}
		var out strings.Builder
		require.NoError(t, route.Render(ctx, params).Render(ctx, &out))
		return route.Title(ctx, params), out.String()
	}
	t.Fatalf("no dock route named %q", name)
	return "", ""
}

func TestTaskTabsServeTheFilesOfTheTaskWorktree(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/files?scope=s1")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "files of task:"+task.ID.String())
	assert.Contains(t, body, `id="task-tabs-s1"`)
	assert.Equal(t, 1, strings.Count(body, `aria-selected="true"`))
}

func TestTaskTabsServeTheChangesOfTheTaskWorktree(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/changes")

	assert.Contains(t, recorder.Body.String(), "changes of task:"+task.ID.String())
}

// committedDiffFake answers the committed diff of the task, or fails.
type committedDiffFake struct {
	diff models.TaskDiff
	err  error
}

func (f committedDiffFake) Execute(_ context.Context, _ models.TaskID) (models.TaskDiff, error) {
	return f.diff, f.err
}

const committedPatch = "diff --git a/internal/a.go b/internal/a.go\nnew file mode 100644\n--- /dev/null\n+++ b/internal/a.go\n@@ -0,0 +1,2 @@\n+one\n+two\n"

func TestTaskChangesTabOpensOnTheUncommittedSubTabWithItsStrip(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	body := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/changes?scope=s1").Body.String()

	assert.Contains(t, body, "changes of task:"+task.ID.String())
	assert.Contains(t, body, `hx-get="/tasks/`+task.ID.String()+`/changes/committed?scope=s1"`)
	assert.Contains(t, body, "Committed")
}

func TestTaskChangesCommittedSubTabListsCommitsAndFiles(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})
	h.uc.CommittedDiff = committedDiffFake{diff: models.TaskDiff{Commits: []string{"abc123 add a"}, Patch: committedPatch}}

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/changes/committed")

	body := recorder.Body.String()
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "abc123 add a")
	assert.Contains(t, body, `data-path="internal/a.go"`)
	assert.Contains(t, body, "/tasks/"+task.ID.String()+"/files/internal/a.go")
	assert.Contains(t, body, "+2")
	assert.NotContains(t, body, "changes of task:")
}

func TestTaskChangesCommittedSubTabSaysWhenThereAreNoCommits(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})
	h.uc.CommittedDiff = committedDiffFake{}

	body := serveTabs(t, h, i18n.Es, "/tasks/"+task.ID.String()+"/changes/committed").Body.String()

	assert.Contains(t, body, "Sin commits respecto a la rama base.")
}

func TestTaskChangesCommittedSubTabHidesTheInternalReadError(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})
	h.uc.CommittedDiff = committedDiffFake{err: errors.New("fatal: secret internal path")}

	body := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/changes/committed").Body.String()

	assert.Contains(t, body, "The diff of the task could not be read.")
	assert.NotContains(t, body, "secret internal path")
}

func TestTaskChangesRefusesASubTabTheStripDoesNotShow(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/changes/other")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestTaskTabsServeTheMergeRequestPanelOfTheTask(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/code-host")

	assert.Contains(t, recorder.Body.String(), "merge-request of task:"+task.ID.String())
}

func TestTaskTabsServeTheActionsOfTheTask(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/actions")

	body := recorder.Body.String()
	assert.Contains(t, body, "/tasks/"+task.ID.String()+"/abort", "a running task offers to abort")
	assert.NotContains(t, body, "files of task:")
}

func TestTaskTabsRefuseATabTheStripDoesNotShow(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/terminal")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestTaskTabsSayThatATaskIsGone(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, werrors.NotFound(task.ID.String()), tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/files")

	assert.NotContains(t, recorder.Body.String(), "files of task:")
	assert.Contains(t, recorder.Body.String(), "no longer exists")
}

func TestTaskTabsSayThatTheFilesAreUnavailableWithoutPanels(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, nil)

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/changes")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "not available")
}

func TestDetailTabOpensOnTheActionsTabAndNamesTheOthers(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	_, body := renderTabsRoute(t, h, TaskDetailRouteName, dock.Params{"id": task.ID.String()})

	assert.Contains(t, body, `hx-get="/tasks/`+task.ID.String()+`/tabs/files?scope=`)
	assert.Contains(t, body, `hx-get="/tasks/`+task.ID.String()+`/tabs/changes?scope=`)
	// One selected tab in the strip of the page, and one in the strip of the kept Changes pane.
	assert.Equal(t, 2, strings.Count(body, `aria-selected="true"`))
}

func TestTaskTabsAnswerAnInvalidTaskWithoutItsTabs(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/not-a-uuid/tabs/files")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), `id="task-tabs"`)
	assert.NotContains(t, recorder.Body.String(), "files of ")
}

// A read failure of the task is answered in the request language, without the internal text of the error.
func TestTaskTabsShowAReadErrorInsteadOfTheTab(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, errors.New("disk on fire"), tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.Fr, "/tasks/"+task.ID.String()+"/tabs/changes")

	body := recorder.Body.String()
	assert.Contains(t, body, "La tâche n’a pas pu être lue.")
	assert.NotContains(t, body, "disk on fire")
	assert.NotContains(t, body, `id="task-tabs"`)
	assert.NotContains(t, body, "changes of ")
}

func TestTaskTabsDropAScopeThatIsNotAnInstance(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/files?scope=%3Cb%3E")

	body := recorder.Body.String()
	assert.Contains(t, body, `id="task-tabs"`, "a scope that is not an instance id falls back to the plain region")
	assert.NotContains(t, body, "%3Cb%3E")
	assert.NotContains(t, body, "<b>")
}

func TestTaskTabsSayThatTheFilesAreUnavailableWithoutPanelsInTheFileTab(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, nil)
	params := dock.Params{"id": task.ID.String(), "path": "src/a.rs"}

	_, filesBody := renderTabsRoute(t, h, TaskFilesRouteName, params)
	_, fileBody := renderTabsRoute(t, h, TaskFileRouteName, params)

	assert.Contains(t, filesBody, "not available")
	assert.Contains(t, fileBody, "not available")
}

func TestTaskFileDockTabsShowTheTreeAndTheFileOfTheTask(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})
	params := dock.Params{"id": task.ID.String(), "path": "src/a.rs"}

	filesTitle, filesBody := renderTabsRoute(t, h, TaskFilesRouteName, params)
	fileTitle, fileBody := renderTabsRoute(t, h, TaskFileRouteName, params)

	assert.Equal(t, "Files", filesTitle)
	assert.Contains(t, filesBody, "tree of task:"+task.ID.String())
	assert.Equal(t, "a.rs", fileTitle, "a file tab takes the name of the file")
	assert.Contains(t, fileBody, "file src/a.rs of task:"+task.ID.String())
}

// The live refresh of the detail is the transcript: a task event swaps only the body of the transcript. Its answer has
// no tab region in it, so the tab that is open is not replaced by the refresh.
func TestTranscriptRefreshCarriesNoTabRegion(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/transcript")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), `id="task-tabs`)
}
