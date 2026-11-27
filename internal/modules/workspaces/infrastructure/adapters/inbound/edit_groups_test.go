package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memberDirectory lists the given groups and puts workspace w1 in g2; it fails with err when set.
type memberDirectory struct {
	groups []GroupInfo
	err    error
}

func (d memberDirectory) Directory(context.Context) ([]GroupInfo, map[string]string, error) {
	return d.groups, map[string]string{"w1": "g2"}, d.err
}

func (memberDirectory) Assign(context.Context, string, string) error { return nil }

func getEditGroups(t *testing.T, groups GroupDirectory) *httptest.ResponseRecorder {
	t.Helper()
	handlers := NewHandlers(UseCases{Groups: groups})
	router := chi.NewRouter()
	handlers.Mount(router)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/workspaces/w1/edit/groups", nil))
	return rec
}

func TestEditGroupsAnswersTheSelectorWithTheCurrentGroupSelected(t *testing.T) {
	rec := getEditGroups(t, memberDirectory{groups: []GroupInfo{{ID: "g1", Name: "Backend"}, {ID: "g2", Name: "Docs"}}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `<option value="g2" selected>Docs</option>`)
	assert.Contains(t, rec.Body.String(), `hx-post="/workspace-groups/workspaces/w1/group"`)
}

func TestEditGroupsLeavesAnEmptySlotWithoutGroupsDirectoryOrOnError(t *testing.T) {
	cases := map[string]GroupDirectory{
		"no groups":       memberDirectory{},
		"directory fails": memberDirectory{groups: []GroupInfo{{ID: "g1", Name: "Backend"}}, err: errors.New("boom")},
		"no directory":    nil,
	}
	for name, groups := range cases {
		t.Run(name, func(t *testing.T) {
			rec := getEditGroups(t, groups)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.NotContains(t, rec.Body.String(), "data-workspace-group-select")
			assert.Contains(t, rec.Body.String(), `id="workspace-edit-groups"`)
		})
	}
}
