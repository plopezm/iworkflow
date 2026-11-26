package inbound_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	httpinbound "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
)

type lazyTasks struct {
	outbound.TaskRepository
	task *models.Task
}

func (f lazyTasks) Find(context.Context, models.TaskID) (*models.Task, error) { return f.task, nil }

type lazyWorktree struct {
	outbound.Worktree
	branches []models.BranchOption
	err      error
}

func (f lazyWorktree) ListBranches(context.Context, string) ([]models.BranchOption, error) {
	return f.branches, f.err
}

type lazyCatalog struct {
	info outbound.AgentCatalogInfo
	err  error
}

func (f lazyCatalog) CatalogForRuntime(context.Context, string) (outbound.AgentCatalogInfo, error) {
	return f.info, f.err
}

// lazyGet serves target on the mounted board handlers, the way the htmx load of the edit page does.
func lazyGet(t *testing.T, tasks lazyTasks, worktree lazyWorktree, catalog lazyCatalog, target string) string {
	t.Helper()
	workspaces := sidebarWorkspaces{refs: []models.WorkspaceRef{{ID: "w1", Name: "api", Path: "/repos/api"}}}
	lifecycle := application.NewTaskLifecycle(tasks, nil, lazyWorkspaceFinder{workspaces}, time.Now)
	uc := httpinbound.BoardUseCases{
		Get:          application.NewGetTask(lifecycle),
		ListBranches: application.NewListWorkspaceBranches(lifecycle, worktree),
		Workspaces:   workspaces,
		AgentCatalog: catalog,
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

// lazyWorkspaceFinder finds the workspaces of the list by id.
type lazyWorkspaceFinder struct{ sidebarWorkspaces }

func (f lazyWorkspaceFinder) Find(_ context.Context, id string) (*models.WorkspaceRef, error) {
	for _, ref := range f.refs {
		if ref.ID == id {
			return &ref, nil
		}
	}
	return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
}

func lazyTask(t *testing.T) lazyTasks {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	runtime, err := models.ParseRuntimeID("claude")
	require.NoError(t, err)
	thinking := "high"
	return lazyTasks{task: &models.Task{ID: id, WorkspaceID: "w1", Agent: models.TaskAgentOptions{Runtime: &runtime, Thinking: &thinking}}}
}

func TestAgentSectionServesTheCatalogWithTheStoredChoices(t *testing.T) {
	t.Parallel()
	tasks := lazyTask(t)
	catalog := lazyCatalog{info: outbound.AgentCatalogInfo{
		Runtimes: []string{"pi", "claude"}, SupportsThinking: true, ThinkingLevels: []string{"low", "high"},
	}}

	html := lazyGet(t, tasks, lazyWorktree{}, catalog, "/board/tasks/agent-section?scope=abc123&task_id="+tasks.task.ID.String())

	assert.Contains(t, html, `id="task-agent-section-abc123"`)
	assert.Regexp(t, `value="claude"[^>]*selected`, html, "the stored runtime stays selected")
	assert.Regexp(t, `value="high"[^>]*selected`, html, "the stored thinking level stays selected")
	assert.NotContains(t, html, `hx-trigger="load"`, "the loaded section does not load again")
}

func TestAgentSectionKeepsTheStoredChoicesWhenTheCatalogFails(t *testing.T) {
	t.Parallel()
	tasks := lazyTask(t)

	html := lazyGet(t, tasks, lazyWorktree{}, lazyCatalog{err: errors.New("agent broke")}, "/board/tasks/agent-section?scope=abc123&task_id="+tasks.task.ID.String())

	assert.Contains(t, html, `name="agent_runtime"`)
	assert.NotContains(t, html, "agent broke")
}

func TestAgentSectionRefusesABadTaskID(t *testing.T) {
	t.Parallel()

	html := lazyGet(t, lazyTask(t), lazyWorktree{}, lazyCatalog{}, "/board/tasks/agent-section?scope=abc123&task_id=nope")

	assert.Contains(t, html, "text-error")
	assert.NotContains(t, html, `name="agent_runtime"`)
}

func branchesTarget(baseBranch string) string {
	query := url.Values{"slot": {"edit"}, "workspace_id": {"w1"}, "scope": {"abc123"}, "base_branch": {baseBranch}}
	return "/board/tasks/branches?" + query.Encode()
}

func TestEditBranchesKeepTheStoredBranchSelected(t *testing.T) {
	t.Parallel()
	worktree := lazyWorktree{branches: []models.BranchOption{{Name: "main", Current: true}, {Name: "origin/develop", Remote: true}}}

	html := lazyGet(t, lazyTask(t), worktree, lazyCatalog{}, branchesTarget("origin/develop"))

	assert.Contains(t, html, `id="task-edit-branch-abc123"`)
	assert.Contains(t, html, `<option value="origin/develop" selected`)
	assert.Contains(t, html, `<option value="main"`)
}

func TestEditBranchesKeepTheStoredBranchAndShowTheErrorWhenListingFails(t *testing.T) {
	t.Parallel()

	html := lazyGet(t, lazyTask(t), lazyWorktree{err: errors.New("git broke")}, lazyCatalog{}, branchesTarget("origin/develop"))

	assert.Contains(t, html, `<option value="origin/develop" selected`, "a save does not lose the stored branch")
	assert.Contains(t, html, "git broke")
	assert.Contains(t, html, "text-error")
}

func TestCreateBranchesStillReplaceTheSelectWithoutAStoredBranch(t *testing.T) {
	t.Parallel()
	worktree := lazyWorktree{branches: []models.BranchOption{{Name: "main", Current: true}}}

	html := lazyGet(t, lazyTask(t), worktree, lazyCatalog{}, "/board/tasks/branches?workspace_id=w1&scope=abc123")

	assert.Contains(t, html, `id="board-branch-slot-abc123"`)
	assert.Contains(t, html, `<option value="main"`)
	assert.NotContains(t, html, "selected>main")
}

func TestCreateBranchesShowTheFailureInPlaceOfTheSelect(t *testing.T) {
	t.Parallel()

	html := lazyGet(t, lazyTask(t), lazyWorktree{err: errors.New("git broke")}, lazyCatalog{}, "/board/tasks/branches?workspace_id=w1&scope=abc123")

	assert.Contains(t, html, "git broke")
	assert.NotContains(t, html, `name="base_branch"`)
}

func TestCreateAgentSectionServesTheCatalogWithoutATask(t *testing.T) {
	t.Parallel()
	catalog := lazyCatalog{info: outbound.AgentCatalogInfo{Runtimes: []string{"pi", "claude"}, SupportsThinking: true, ThinkingLevels: []string{"low", "high"}}}

	html := lazyGet(t, lazyTask(t), lazyWorktree{}, catalog, "/board/tasks/agent-section?scope=abc123&agent_runtime=claude&agent_thinking=high")

	assert.Contains(t, html, `id="task-agent-section-abc123"`)
	assert.Regexp(t, `value="claude"[^>]*selected`, html, "the runtime of the form stays selected")
	assert.Regexp(t, `value="high"[^>]*selected`, html, "the thinking level of the form stays selected")
	assert.NotContains(t, html, `hx-trigger="load"`, "the loaded section does not load again")
}

func TestCreateBranchesKeepThePickedBranchSelected(t *testing.T) {
	t.Parallel()
	worktree := lazyWorktree{branches: []models.BranchOption{{Name: "main", Current: true}, {Name: "origin/develop", Remote: true}}}

	html := lazyGet(t, lazyTask(t), worktree, lazyCatalog{}, "/board/tasks/branches?workspace_id=w1&scope=abc123&base_branch=origin%2Fdevelop")

	assert.Contains(t, html, `id="board-branch-slot-abc123"`)
	assert.Contains(t, html, `<option value="origin/develop" selected`)
}

func TestCreateWorkspacesServeTheSelectAndTheBranchPlaceholder(t *testing.T) {
	t.Parallel()

	html := lazyGet(t, lazyTask(t), lazyWorktree{}, lazyCatalog{}, "/board/tasks/create-workspaces?scope=abc123&workspace_id=w1&base_branch=dev")

	assert.Contains(t, html, `id="board-workspace-fields-abc123"`)
	assert.Regexp(t, `value="w1"[^>]*selected`, html, "the workspace of the form stays selected")
	assert.Contains(t, html, `/board/tasks/branches?`, "the branch list loads next")
	assert.NotContains(t, html, `/board/tasks/create-workspaces`, "the loaded fields do not load again")
}

// brokenWorkspaces fails to list the workspaces; Find is unused by the create-workspaces fragment.
type brokenWorkspaces struct{ sidebarWorkspaces }

func (brokenWorkspaces) List(context.Context) ([]models.WorkspaceRef, error) {
	return nil, errors.New("registry down")
}

func TestCreateWorkspacesShowTheFailureInPlaceOfTheFields(t *testing.T) {
	t.Parallel()
	workspaces := brokenWorkspaces{}
	uc := httpinbound.BoardUseCases{Workspaces: workspaces}
	router := chi.NewRouter()
	httpinbound.NewBoardHandlers(uc, time.Now).Mount(router)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	req := httptest.NewRequest(http.MethodGet, "/board/tasks/create-workspaces?scope=abc123", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, `id="board-workspace-fields-abc123"`, "the error takes the place of the slot, so the slot stays addressable")
	assert.Contains(t, html, "registry down")
	assert.NotContains(t, html, `name="workspace_id"`, "no form field is left to submit")
}
