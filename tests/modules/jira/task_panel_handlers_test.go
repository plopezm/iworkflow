package jira_test

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/di"
)

// stubTaskInfo answers the task a worklog is proposed from.
type stubTaskInfo struct {
	snapshot outbound.TaskSnapshot
}

var _ outbound.TaskInfo = (*stubTaskInfo)(nil)

func (s *stubTaskInfo) Info(_ context.Context, _ string) (outbound.TaskSnapshot, error) {
	return s.snapshot, nil
}

// taskPanelFixture drives the Jira panel of task-1 through the composition root, against a fake Jira server.
type taskPanelFixture struct {
	t        *testing.T
	router   chi.Router
	module   *di.Module
	fakeJira *fakeJira
}

const panelTaskID = "task-1"

func newTaskPanelFixture(t *testing.T, connected bool) *taskPanelFixture {
	t.Helper()
	fake := newFakeJira(t)
	module := di.NewModule(di.Deps{
		DB:         dbtest.New(t),
		Tasks:      &stubTasks{},
		Workspaces: stubWorkspaces{},
		TaskInfo: &stubTaskInfo{snapshot: outbound.TaskSnapshot{
			Title:        "Migrar login",
			Finished:     true,
			AgentRunTime: 80 * time.Minute,
			Commits:      []string{"abc1234 add login"},
		}},
	})
	if connected {
		_, err := module.SaveConnection.Execute(context.Background(), application.SaveConnectionInput{
			BaseURL: fake.url(),
			Token:   testToken,
		})
		require.NoError(t, err)
	}
	router := chi.NewRouter()
	module.Routes(router)
	return &taskPanelFixture{t: t, router: router, module: module, fakeJira: fake}
}

func (f *taskPanelFixture) ctx() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
}

// send asks the panel like htmx does: a form post, or a plain get when values is nil.
func (f *taskPanelFixture) send(method string, path string, values url.Values) string {
	f.t.Helper()
	var body *strings.Reader
	if values != nil {
		body = strings.NewReader(values.Encode())
	} else {
		body = strings.NewReader("")
	}
	request := httptest.NewRequest(method, path, body).WithContext(f.ctx())
	request.Header.Set("HX-Request", "true")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	require.Equal(f.t, http.StatusOK, recorder.Code)
	return recorder.Body.String()
}

func (f *taskPanelFixture) link(key string) string {
	return f.send(http.MethodPost, "/jira/tasks/"+panelTaskID+"/link", url.Values{"key": {key}})
}

func (f *taskPanelFixture) worklog(timeText string, description string, alsoComment bool) string {
	values := url.Values{"time": {timeText}, "description": {description}}
	if alsoComment {
		values.Set("also_comment", "on")
	}
	return f.send(http.MethodPost, "/jira/tasks/"+panelTaskID+"/worklog", values)
}

// countWrites counts the requests that change something in Jira: every method but GET.
func (f *taskPanelFixture) countWrites() int {
	count := 0
	for _, call := range f.fakeJira.requests() {
		if call.method != http.MethodGet {
			count++
		}
	}
	return count
}

// countRequests counts the requests the fake Jira got with the method and path suffix.
func (f *taskPanelFixture) countRequests(method string, suffix string) int {
	count := 0
	for _, call := range f.fakeJira.requests() {
		if call.method == method && strings.HasSuffix(call.path, suffix) {
			count++
		}
	}
	return count
}

func TestTaskPanelOffersTheLinkFieldWhenTheTaskHasNoIssue(t *testing.T) {
	f := newTaskPanelFixture(t, true)

	body := f.send(http.MethodGet, "/jira/tasks/"+panelTaskID+"/panel", nil)

	assert.Contains(t, body, `id="jira-task-panel-task-1"`)
	assert.Contains(t, body, `hx-post="/jira/tasks/task-1/link"`)
	assert.NotContains(t, body, `name="time"`, "the worklog needs a linked issue")
}

func TestTaskPanelLinksAnIssueByKeyAndShowsIt(t *testing.T) {
	f := newTaskPanelFixture(t, true)

	body := f.link(" b-7 ")

	linked, err := f.module.GetTaskIssue.Execute(context.Background(), panelTaskID)
	require.NoError(t, err)
	require.NotNil(t, linked)
	assert.Equal(t, "B-7", linked.Key())
	assert.Contains(t, body, "resumen B-7")
	assert.Contains(t, body, `hx-post="/jira/tasks/task-1/refresh"`)
	assert.Contains(t, body, `hx-post="/jira/tasks/task-1/worklog"`)
	assert.Zero(t, f.countRequests(http.MethodPost, "/worklog"), "linking never writes a worklog")
}

func TestTaskPanelRefusesAnInvalidKeyInlineAndKeepsIt(t *testing.T) {
	f := newTaskPanelFixture(t, true)

	body := f.link("../x")

	linked, err := f.module.GetTaskIssue.Execute(context.Background(), panelTaskID)
	require.NoError(t, err)
	assert.Nil(t, linked)
	assert.Contains(t, body, html.EscapeString(jiraerrors.InvalidIssueKey("../X").Error()))
	assert.Contains(t, body, `value="../x"`)
}

func TestTaskPanelWorklogRegistersTheTimeAndTheLocalRecord(t *testing.T) {
	f := newTaskPanelFixture(t, true)
	f.link("B-7")

	body := f.worklog("1h 30m", "Migrar login\n\n- add login", true)

	require.Equal(t, 1, f.countRequests(http.MethodPost, "/rest/api/2/issue/B-7/worklog"))
	assert.Equal(t, 1, f.countRequests(http.MethodPost, "/rest/api/2/issue/B-7/comment"), "the comment is optional and was checked")
	worklogCall := f.findRequest(http.MethodPost, "/worklog")
	assert.Contains(t, worklogCall.body, `"timeSpentSeconds":5400`)

	logged, err := f.module.ListTaskWorklogs.Execute(context.Background(), panelTaskID)
	require.NoError(t, err)
	require.Len(t, logged.Records, 1)
	assert.Equal(t, "777", logged.Records[0].JiraID)
	assert.Equal(t, 90*time.Minute, logged.Total)

	assert.Contains(t, body, "Registrado: 1h 30m")
	assert.Contains(t, body, "No queda tiempo por registrar.", "80 min proposed, 90 min logged: nothing is left")
	assert.Contains(t, body, `class="btn-primary" disabled>`, "the register button is disabled when nothing is left")
}

func TestTaskPanelWorklogWithInvalidTimeShowsTheErrorAndChangesNothing(t *testing.T) {
	f := newTaskPanelFixture(t, true)
	f.link("B-7")
	writesBefore := f.countWrites()

	body := f.worklog("soon", "Migrar login", true)

	assert.Contains(t, body, html.EscapeString(jiraerrors.InvalidDuration("soon").Error()))
	assert.Contains(t, body, `value="soon"`, "the typed time is kept for the user to fix it")
	assert.Zero(t, f.countRequests(http.MethodPost, "/worklog"))
	assert.Zero(t, f.countRequests(http.MethodPost, "/comment"))
	assert.Equal(t, writesBefore, f.countWrites(), "nothing is written to Jira; the panel only reads")
	logged, err := f.module.ListTaskWorklogs.Execute(context.Background(), panelTaskID)
	require.NoError(t, err)
	assert.Empty(t, logged.Records, "no local record is stored")
}

func TestTaskPanelRefreshReadsTheIssueFromJiraAgain(t *testing.T) {
	f := newTaskPanelFixture(t, true)
	f.link("B-7")
	before := f.countRequests(http.MethodGet, "/rest/api/2/issue/B-7")

	body := f.send(http.MethodPost, "/jira/tasks/"+panelTaskID+"/refresh", url.Values{})

	assert.Equal(t, before+1, f.countRequests(http.MethodGet, "/rest/api/2/issue/B-7"))
	assert.Contains(t, body, "resumen B-7")
}

func TestTaskPanelTransitionMovesTheIssueAndRejectsAnEmptyChoice(t *testing.T) {
	f := newTaskPanelFixture(t, true)
	f.link("B-7")

	empty := f.send(http.MethodPost, "/jira/tasks/"+panelTaskID+"/transition", url.Values{"transition_id": {""}})
	assert.Contains(t, empty, "Elige un estado.")
	assert.Zero(t, f.countRequests(http.MethodPost, "/transitions"))

	body := f.send(http.MethodPost, "/jira/tasks/"+panelTaskID+"/transition", url.Values{"transition_id": {"31"}})

	assert.Equal(t, 1, f.countRequests(http.MethodPost, "/transitions"))
	assert.Contains(t, body, "Cerrar → Done")
	assert.NotContains(t, body, "Elige un estado.")
}

func TestTaskPanelWithoutAConnectionOffersNoTabAndNoLink(t *testing.T) {
	f := newTaskPanelFixture(t, false)

	visible, component := f.module.TaskPanel(context.Background(), panelTaskID)

	assert.False(t, visible, "without a Jira connection the Jira tab is not offered")
	assert.Nil(t, component)
}

func TestTaskPanelIsOfferedWithAConnection(t *testing.T) {
	f := newTaskPanelFixture(t, true)

	visible, component := f.module.TaskPanel(context.Background(), panelTaskID)

	require.True(t, visible)
	require.NotNil(t, component)
	body := renderWith(t, component, f.ctx())
	assert.Contains(t, body, `id="jira-task-panel-task-1"`)
	assert.Contains(t, body, `hx-post="/jira/tasks/task-1/link"`)
}

func TestTaskPanelWithoutTaskInfoShowsTheProposalErrorInline(t *testing.T) {
	fake := newFakeJira(t)
	module := di.NewModule(di.Deps{DB: dbtest.New(t), Tasks: &stubTasks{}, Workspaces: stubWorkspaces{}})
	_, err := module.SaveConnection.Execute(context.Background(), application.SaveConnectionInput{BaseURL: fake.url(), Token: testToken})
	require.NoError(t, err)
	_, err = module.LinkTaskToIssue.Execute(context.Background(), panelTaskID, "B-7")
	require.NoError(t, err)
	router := chi.NewRouter()
	module.Routes(router)

	request := httptest.NewRequest(http.MethodGet, "/jira/tasks/"+panelTaskID+"/panel", nil).WithContext(
		i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "resumen B-7", "the issue still shows when the task cannot be read")
	assert.Contains(t, recorder.Body.String(), "no está disponible", "the missing task data is said inline")
	assert.NotContains(t, recorder.Body.String(), "No queda tiempo por registrar.", "a failed read is not reported as nothing left to log")
}

// renderWith renders a component with the given context and returns its HTML.
func renderWith(t *testing.T, component templ.Component, ctx context.Context) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

// findRequest returns the first request the fake Jira got with the method and path suffix.
func (f *taskPanelFixture) findRequest(method string, suffix string) received {
	for _, call := range f.fakeJira.requests() {
		if call.method == method && strings.HasSuffix(call.path, suffix) {
			return call
		}
	}
	f.t.Fatalf("no %s request ending in %q", method, suffix)
	return received{}
}
