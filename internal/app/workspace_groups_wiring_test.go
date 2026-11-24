package app_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceGroupsAreMountedAndTheirGroupsShowInTheSidebar(t *testing.T) {
	handler := newRouter(t)
	form := url.Values{"name": {"Backend"}}
	request := httptest.NewRequest(http.MethodPost, "/workspace-groups", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, request)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "board:refresh", rec.Header().Get("HX-Trigger"))
	body := get(t, handler, "/board/sidebar").Body.String()
	assert.Contains(t, body, `data-group-id="`, "the created group is arranged into the sidebar")
	assert.Contains(t, body, "Backend")
}

func TestGroupsHaveTheirOwnScreensUnderWorkspaces(t *testing.T) {
	handler := newRouter(t)
	form := url.Values{"name": {"Family"}}
	create := httptest.NewRequest(http.MethodPost, "/workspace-groups", strings.NewReader(form.Encode()))
	create.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), create)
	match := regexp.MustCompile(`data-group-id="([^"]+)"`).FindStringSubmatch(get(t, handler, "/workspace-groups/list").Body.String())
	require.Len(t, match, 2, "the created group is listed")
	id := match[1]

	workspaces := get(t, handler, "/dock/pane?path=/workspaces").Body.String()
	assert.NotContains(t, workspaces, "group-manager", "Workspaces no longer manages groups")
	assert.NotContains(t, workspaces, "workspace-groups/manage")
	assert.Contains(t, workspaces, `data-dock-path="/workspaces/groups"`, "Workspaces links to the groups screen")

	list := get(t, handler, "/dock/pane?path=/workspaces/groups").Body.String()
	assert.Contains(t, list, `hx-post="/workspace-groups"`)
	assert.Contains(t, list, `data-dock-path="/workspaces"`, "the breadcrumb leads back to Workspaces")

	detail := get(t, handler, "/dock/pane?path=/workspaces/groups/"+id).Body.String()
	assert.Contains(t, detail, `value="Family"`)
	assert.Contains(t, detail, `data-dock-path="/workspaces/groups"`, "the breadcrumb leads back to Groups")
	assert.Contains(t, detail, "/workspace-groups/"+id+"/members")
}
