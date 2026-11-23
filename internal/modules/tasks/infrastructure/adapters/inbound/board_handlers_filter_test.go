package inbound_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	httpinbound "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
)

// filterValuePattern is the typed text of the search input.
var filterValuePattern = regexp.MustCompile(`(id="board-sidebar-filter"[^>]*?) value="[^"]*"`)

// sidebarFilterResponse answers GET target on the mounted board handlers, the way the sidebar's htmx request does.
func sidebarFilterResponse(t *testing.T, target string) string {
	t.Helper()
	uc := httpinbound.BoardUseCases{
		List:       application.NewListTasks(sidebarTasks{}),
		Workspaces: sidebarWorkspaces{refs: []models.WorkspaceRef{{ID: "w1", Name: "api"}, {ID: "w2", Name: "web"}}},
	}
	router := chi.NewRouter()
	httpinbound.NewBoardHandlers(uc, time.Now).Mount(router)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestBoardSidebarQueryNarrowsTheWorkspaces(t *testing.T) {
	t.Parallel()

	html := sidebarFilterResponse(t, "/board/sidebar?q=API")

	assert.Contains(t, html, `data-workspace-row="w1"`, "the workspace whose name matches stays")
	assert.NotContains(t, html, `data-workspace-row="w2"`, "the workspace whose name does not match is hidden")
}

func TestBoardSidebarQueryWithNoMatchShowsTheEmptyLine(t *testing.T) {
	t.Parallel()

	html := sidebarFilterResponse(t, "/board/sidebar?q=zzz")

	assert.NotContains(t, html, `data-workspace-row=`)
	assert.Contains(t, html, "No workspaces match.")
}

func TestBoardSidebarWithoutAQueryIsUnfiltered(t *testing.T) {
	t.Parallel()

	plain := sidebarFilterResponse(t, "/board/sidebar")
	blank := sidebarFilterResponse(t, "/board/sidebar?q=%20%20")

	assert.Contains(t, plain, `data-workspace-row="w1"`)
	assert.Contains(t, plain, `data-workspace-row="w2"`)
	assert.NotContains(t, plain, "No workspaces match.")
	assert.Equal(t, filterValuePattern.ReplaceAllString(plain, "$1"), filterValuePattern.ReplaceAllString(blank, "$1"),
		"a blank query renders the same rows as no query")
	assert.Contains(t, blank, ` value="  "`, "the input echoes the blank query as typed")
}
