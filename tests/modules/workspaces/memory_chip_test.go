package workspaces_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
	"iworkflow/internal/modules/workspaces/infrastructure/di"
)

// memoryPortFake answers the memory summaries of one batch per name. Names without a summary are unavailable,
// and err, when set, fails the whole batch instead.
type memoryPortFake struct {
	summaries map[string]models.MemorySummary
	err       error
}

func (f memoryPortFake) Summaries(_ context.Context, workspaces []models.Workspace) ([]models.MemorySummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]models.MemorySummary, 0, len(workspaces))
	for _, workspace := range workspaces {
		summary, found := f.summaries[workspace.Name]
		if !found {
			summary = models.MemorySummary{State: models.MemoryUnavailable}
		}
		out = append(out, summary)
	}
	return out, nil
}

// newMemoryFixture is the workspaces module with the given memory port, mounted like the real server. A nil port
// means no Engram is wired.
func newMemoryFixture(t *testing.T, memory outbound.MemorySummaryPort) *httpFixture {
	t.Helper()
	home := t.TempDir()
	module := di.NewModule(di.Deps{
		DB:          dbtest.New(t),
		HomeDir:     home,
		Tasks:       noTasks{},
		AgentsFiles: noAgentsFiles{},
		Memory:      memory,
	})
	mux := chi.NewRouter()
	module.Routes(mux)
	return &httpFixture{t: t, module: module, router: withSpanish(mux), home: home}
}

func TestWorkspaceRowsShowTheEngramChipOfEachState(t *testing.T) {
	fixture := newMemoryFixture(t, memoryPortFake{summaries: map[string]models.MemorySummary{
		"alpha": {State: models.MemoryAvailable, Count: 3},
		"beta":  {State: models.MemoryEmpty},
	}})
	fixture.register("alpha", newGitRepo(t))
	fixture.register("beta", newGitRepo(t))
	fixture.register("gamma", newGitRepo(t))
	tab := fixture.module.Tabs()[0]

	body := renderTab(t, tab.Render(spanishContext(), dock.Params{}))

	assert.Contains(t, body, "Engram · 3 memorias", "available with memories shows the count")
	assert.NotContains(t, body, "Última actividad", "Engram does not expose the last activity, so the chip shows none")
	assert.Contains(t, body, "Engram · sin memorias", "available without memories says so")
	assert.Contains(t, body, "Engram no disponible", "a workspace Engram does not answer for is unavailable")
	assert.Contains(t, body, `class="memory-chip memory-chip-off"`)
}

func TestWorkspacesPageRendersWhenEngramIsUnavailable(t *testing.T) {
	cases := map[string]memoryPortFake{
		"reports unavailable": {},
		"fails with an error": {err: context.DeadlineExceeded},
	}
	for name, memory := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newMemoryFixture(t, memory)
			fixture.register("Demo", newGitRepo(t))
			tab := fixture.module.Tabs()[0]
			request := httptest.NewRequest(http.MethodGet, "/workspaces", nil).WithContext(spanishContext())
			recorder := httptest.NewRecorder()

			templ.Handler(tab.Render(spanishContext(), dock.Params{})).ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "Demo", "the workspace is still listed")
			assert.Contains(t, recorder.Body.String(), "Engram no disponible")
		})
	}
}

func TestWorkspacesPageShowsNoMemoryChipWhenNoProviderIsWired(t *testing.T) {
	fixture := newMemoryFixture(t, nil)
	fixture.register("Demo", newGitRepo(t))
	tab := fixture.module.Tabs()[0]
	request := httptest.NewRequest(http.MethodGet, "/workspaces", nil).WithContext(spanishContext())
	recorder := httptest.NewRecorder()

	templ.Handler(tab.Render(spanishContext(), dock.Params{})).ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Demo", "the workspace is still listed")
	assert.NotContains(t, recorder.Body.String(), "Engram")
	assert.NotContains(t, recorder.Body.String(), "memory-chip")
}
