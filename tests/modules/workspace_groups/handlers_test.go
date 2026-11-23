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
	module := di.NewModule(db)
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
