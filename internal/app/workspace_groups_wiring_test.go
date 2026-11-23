package app_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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
