package jira_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func transitionPosts(f *boardFixture) []received {
	var posts []received
	for _, call := range f.fakeJira.requests() {
		if call.method == http.MethodPost && strings.HasSuffix(call.path, "/transitions") {
			posts = append(posts, call)
		}
	}
	return posts
}

func TestEpicHandlers_StatusMenuListsTheAllowedTransitions(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	req := httptest.NewRequest(http.MethodGet, "/jira/epics/none/issues/A-1/transitions", nil)
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	require.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, `hx-post="/jira/epics/none/issues/A-1/transition"`)
	assert.Contains(t, body, `name="transition_id"`)
	assert.Contains(t, body, `value="31"`)
	assert.Contains(t, body, "Cerrar")
	assert.Contains(t, body, "Done")
}

func TestEpicHandlers_ChangeStatusAppliesTheTransitionAndReturnsTheBoard(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	form := url.Values{"transition_id": {"31"}}
	req := httptest.NewRequest(http.MethodPost, "/jira/epics/none/issues/A-1/transition", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	require.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, `id="jira-board"`)
	assert.Contains(t, body, `aria-current="true"`, "the same epic stays open")
	posts := transitionPosts(f)
	require.Len(t, posts, 1)
	assert.Equal(t, "/rest/api/2/issue/A-1/transitions", posts[0].path)
	assert.JSONEq(t, `{"transition":{"id":"31"}}`, posts[0].body)
}

func TestEpicHandlers_ChangeStatusOfAnUnknownIssueShowsTheErrorAndCallsNothing(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	form := url.Values{"transition_id": {"31"}}
	req := httptest.NewRequest(http.MethodPost, "/jira/epics/none/issues/Z-9/transition", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	require.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, `id="jira-board"`)
	assert.Contains(t, body, "Z-9", "the failure is shown above the board")
	assert.Empty(t, transitionPosts(f), "Jira is not called for an issue that was never imported")
}

func TestEpicHandlers_LaneRoutesAnswerInTheLanesMode(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	req := httptest.NewRequest(http.MethodGet, "/jira/lanes/new", nil)
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "Mis tareas")
}

func TestEpicHandlers_BoardKeepsTheFilterAndTheOpenEpic(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	req := httptest.NewRequest(http.MethodGet, "/jira/board?mode=epics&epic=none&search=A-1&status=To+Do", nil)
	req.Header.Set("HX-Request", "true")
	body := f.serve(req).Body.String()

	assert.Contains(t, body, `<option value="To Do" selected>`)
	assert.Contains(t, body, `value="A-1"`)
	assert.Contains(t, body, `name="epic" value="none"`)
}
