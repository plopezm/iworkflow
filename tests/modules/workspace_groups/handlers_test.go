package workspace_groups_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/workspace_groups/infrastructure/di"
)

func newRouter(t *testing.T) (http.Handler, *di.Module) {
	t.Helper()
	db := dbtest.New(t)
	insertWorkspace(t, db, "w1")
	// These tests post to the action endpoints only; the management card (which needs the workspace catalog) is covered
	// by the inbound package tests.
	module := di.NewModule(db, nil)
	router := chi.NewRouter()
	module.Routes(router)
	return router, module
}

func post(router http.Handler, path string, form url.Values, header map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for key, value := range header {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestCreateGroupAnswers204AndAnnouncesASidebarRefresh(t *testing.T) {
	router, module := newRouter(t)

	response := post(router, "/workspace-groups", url.Values{"name": {"Backend"}, "color": {"blue"}}, nil)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "board:refresh", response.Header().Get("HX-Trigger"))
	layout, err := module.Arrange.Execute(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, layout.Groups, 1)
	require.Equal(t, "Backend", layout.Groups[0].Group.Name())
}

func TestCreateGroupTakesTheNameFromTheHtmxPromptHeader(t *testing.T) {
	router, module := newRouter(t)

	response := post(router, "/workspace-groups", url.Values{}, map[string]string{"HX-Prompt": "  From prompt "})

	require.Equal(t, http.StatusNoContent, response.Code)
	layout, _ := module.Arrange.Execute(context.Background(), nil)
	require.Equal(t, "From prompt", layout.Groups[0].Group.Name())
}

func TestInvalidGroupIs400AndStoresNothing(t *testing.T) {
	router, module := newRouter(t)

	blank := post(router, "/workspace-groups", url.Values{"name": {"  "}}, nil)
	badColor := post(router, "/workspace-groups", url.Values{"name": {"a"}, "color": {"nope"}}, nil)

	require.Equal(t, http.StatusBadRequest, blank.Code)
	require.Equal(t, http.StatusBadRequest, badColor.Code)
	require.Empty(t, blank.Header().Get("HX-Trigger"))
	layout, _ := module.Arrange.Execute(context.Background(), nil)
	require.Empty(t, layout.Groups)
}

func TestMoveCollapseRenameAndDeleteAGroup(t *testing.T) {
	router, module := newRouter(t)
	ctx := context.Background()
	group, err := module.Create.Execute(ctx, groupInput("Old"))
	require.NoError(t, err)
	id := group.ID()

	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/workspaces/w1/group", url.Values{"group": {id}}, nil).Code)
	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/"+id+"/collapse", url.Values{"collapsed": {"true"}}, nil).Code)
	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/"+id, url.Values{"name": {"New"}, "color": {"red"}}, nil).Code)
	layout, _ := module.Arrange.Execute(ctx, []string{"w1"})
	require.Equal(t, "New", layout.Groups[0].Group.Name())
	require.Equal(t, "red", layout.Groups[0].Group.Color())
	require.True(t, layout.Groups[0].Group.Collapsed())
	require.Equal(t, []string{"w1"}, layout.Groups[0].Workspaces)

	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/workspaces/w1/group", url.Values{"group": {""}}, nil).Code)
	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/"+id+"/delete", url.Values{}, nil).Code)
	layout, _ = module.Arrange.Execute(ctx, []string{"w1"})
	require.Empty(t, layout.Groups)
	require.Equal(t, []string{"w1"}, layout.Ungrouped)
}

func TestPinAndUnpinAWorkspace(t *testing.T) {
	router, module := newRouter(t)

	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/workspaces/w1/pin", url.Values{"pinned": {"true"}}, nil).Code)
	layout, _ := module.Arrange.Execute(context.Background(), []string{"w1"})
	require.Equal(t, []string{"w1"}, layout.Pinned)
	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/workspaces/w1/pin", url.Values{"pinned": {"false"}}, nil).Code)
	layout, _ = module.Arrange.Execute(context.Background(), []string{"w1"})
	require.Empty(t, layout.Pinned)
}

func TestMissingTargetsAre404WithoutARefresh(t *testing.T) {
	router, _ := newRouter(t)
	for _, tc := range []struct {
		path string
		form url.Values
	}{
		{"/workspace-groups/ghost", url.Values{"name": {"x"}}},
		{"/workspace-groups/ghost/delete", url.Values{}},
		{"/workspace-groups/ghost/collapse", url.Values{"collapsed": {"true"}}},
		{"/workspace-groups/workspaces/ghost/pin", url.Values{"pinned": {"true"}}},
		{"/workspace-groups/workspaces/w1/group", url.Values{"group": {"ghost"}}},
	} {
		response := post(router, tc.path, tc.form, nil)
		require.Equal(t, http.StatusNotFound, response.Code, tc.path)
		require.Empty(t, response.Header().Get("HX-Trigger"), tc.path)
	}
}

// postBody posts a raw form body, for the requests that are not well-formed.
func postBody(router http.Handler, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// The flags are exactly "true" or "false": a missing or garbled one is a 400 that stores nothing and asks for no refresh.
func TestFlagsAreExactlyTrueOrFalseAndAnythingElseIsA400(t *testing.T) {
	router, module := newRouter(t)
	ctx := context.Background()
	group, err := module.Create.Execute(ctx, groupInput("G"))
	require.NoError(t, err)
	id := group.ID()
	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/"+id+"/collapse", url.Values{"collapsed": {"true"}}, nil).Code)
	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/workspaces/w1/pin", url.Values{"pinned": {"true"}}, nil).Code)

	for _, garbled := range []url.Values{{}, {"collapsed": {""}}, {"collapsed": {"1"}}, {"collapsed": {"True"}}, {"collapsed": {"yes"}}} {
		response := post(router, "/workspace-groups/"+id+"/collapse", garbled, nil)
		require.Equal(t, http.StatusBadRequest, response.Code, garbled.Encode())
		require.Empty(t, response.Header().Get("HX-Trigger"), garbled.Encode())
	}
	for _, garbled := range []url.Values{{}, {"pinned": {""}}, {"pinned": {"1"}}, {"pinned": {"True"}}, {"pinned": {"no"}}} {
		response := post(router, "/workspace-groups/workspaces/w1/pin", garbled, nil)
		require.Equal(t, http.StatusBadRequest, response.Code, garbled.Encode())
		require.Empty(t, response.Header().Get("HX-Trigger"), garbled.Encode())
	}

	layout, err := module.Arrange.Execute(ctx, []string{"w1"})
	require.NoError(t, err)
	require.True(t, layout.Groups[0].Group.Collapsed(), "a garbled flag keeps the stored collapsed state")
	require.Equal(t, []string{"w1"}, layout.Pinned, "a garbled flag keeps the stored pin")
}

// A body that cannot be parsed is invalid input, never a server failure, and stores nothing.
func TestMalformedFormBodyIsA400AndStoresNothing(t *testing.T) {
	router, module := newRouter(t)
	ctx := context.Background()
	group, err := module.Create.Execute(ctx, groupInput("G"))
	require.NoError(t, err)
	id := group.ID()

	for _, tc := range []struct {
		path string
		body string
	}{
		{"/workspace-groups", "name=%zz"},
		{"/workspace-groups/" + id, "name=%zz&color=red"},
		{"/workspace-groups/" + id + "/collapse", "collapsed=%zz"},
		{"/workspace-groups/workspaces/w1/group", "group=%zz"},
		{"/workspace-groups/workspaces/w1/pin", "pinned=%zz"},
	} {
		response := postBody(router, tc.path, tc.body)
		require.Equal(t, http.StatusBadRequest, response.Code, tc.path)
		require.Empty(t, response.Header().Get("HX-Trigger"), tc.path)
	}

	layout, err := module.Arrange.Execute(ctx, []string{"w1"})
	require.NoError(t, err)
	require.Len(t, layout.Groups, 1)
	require.Equal(t, "G", layout.Groups[0].Group.Name())
	require.Empty(t, layout.Groups[0].Workspaces)
	require.Empty(t, layout.Pinned)
	require.Equal(t, []string{"w1"}, layout.Ungrouped)
}

// Deleting a group that still holds a workspace frees the workspace: it is listed as ungrouped.
func TestDeletingAGroupWithAWorkspaceKeepsTheWorkspaceUngrouped(t *testing.T) {
	router, module := newRouter(t)
	ctx := context.Background()
	group, err := module.Create.Execute(ctx, groupInput("Backend"))
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, post(router, "/workspace-groups/workspaces/w1/group", url.Values{"group": {group.ID()}}, nil).Code)

	response := post(router, "/workspace-groups/"+group.ID()+"/delete", url.Values{}, nil)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "board:refresh", response.Header().Get("HX-Trigger"))
	layout, err := module.Arrange.Execute(ctx, []string{"w1"})
	require.NoError(t, err)
	require.Empty(t, layout.Groups)
	require.Equal(t, []string{"w1"}, layout.Ungrouped)
}

// A workspace that does not exist cannot be moved into a group that does.
func TestMovingAMissingWorkspaceIntoAnExistingGroupIs404(t *testing.T) {
	router, module := newRouter(t)
	group, err := module.Create.Execute(context.Background(), groupInput("Backend"))
	require.NoError(t, err)

	response := post(router, "/workspace-groups/workspaces/ghost/group", url.Values{"group": {group.ID()}}, nil)

	require.Equal(t, http.StatusNotFound, response.Code)
	require.Empty(t, response.Header().Get("HX-Trigger"))
}
