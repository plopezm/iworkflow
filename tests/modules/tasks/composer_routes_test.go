package tasks_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The composer endpoints must be mounted by the module itself: a page that posts to a route nobody serves leaves the
// message box dead.
func TestModuleMountsTheComposerEndpoints(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	_, router := boardRouter(t, repo)
	id := newIDForTest(t)

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"message", http.MethodPost, "/tasks/" + id + "/messages"},
		{"mode switch", http.MethodPost, "/tasks/" + id + "/mode"},
		{"file suggestions", http.MethodGet, "/tasks/" + id + "/composer/files?q=a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			assert.NotEqual(t, http.StatusNotFound, recorder.Code, "the route is served")
			assert.NotEqual(t, http.StatusMethodNotAllowed, recorder.Code, "the method is served")
		})
	}
}
