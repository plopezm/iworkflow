package tasks_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/di"
)

// listingLookup reads every workspace, so the switcher and the workspace-scoped lists see them all.
type listingLookup struct {
	workspaceLookup
}

func (l listingLookup) List(ctx context.Context) ([]models.WorkspaceRef, error) {
	rows, err := l.repo.DB.QueryContext(ctx, "SELECT id, name, path, base_branch FROM workspaces ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var refs []models.WorkspaceRef
	for rows.Next() {
		var ref models.WorkspaceRef
		if err := rows.Scan(&ref.ID, &ref.Name, &ref.Path, &ref.BaseBranch); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// failingLookup fails every workspace read, as an unreadable workspace registry would.
type failingLookup struct{}

func (failingLookup) Find(context.Context, string) (*models.WorkspaceRef, error) {
	return nil, errors.New("workspace registry unavailable")
}

func (failingLookup) List(context.Context) ([]models.WorkspaceRef, error) {
	return nil, errors.New("workspace registry unavailable")
}

// breakTaskTable makes every read of the tasks fail, as a damaged database would.
func breakTaskTable(t *testing.T, repo *Repo) {
	t.Helper()
	_, err := repo.DB.ExecContext(context.Background(), "ALTER TABLE tasks RENAME TO tasks_damaged")
	require.NoError(t, err)
}

// boardRouter composes the module with a listing workspace lookup and mounts its routes.
func boardRouter(t *testing.T, repo *Repo) (*di.Module, chi.Router) {
	t.Helper()
	return boardRouterWith(t, repo, listingLookup{workspaceLookup{repo: repo}})
}

// boardRouterWith composes the module with the given workspace lookup and mounts its routes.
func boardRouterWith(t *testing.T, repo *Repo, workspaces outbound.WorkspaceLookup) (*di.Module, chi.Router) {
	t.Helper()
	module, err := di.NewModule(di.Deps{
		DB:             repo.DB,
		Workspaces:     workspaces,
		AttachmentsDir: t.TempDir(),
		WorktreesDir:   t.TempDir(),
		Binaries:       testBinaries,
		LaunchSettings: stubSettings{},
	})
	require.NoError(t, err)
	t.Cleanup(module.ShutdownEvents)
	router := chi.NewRouter()
	module.Routes(router)
	return module, router
}

// renderTab renders the component of a dock route the module declares.
func renderTab(t *testing.T, module *di.Module, name string, params dock.Params) string {
	t.Helper()
	for _, route := range module.Tabs() {
		if route.Name != name {
			continue
		}
		var out bytes.Buffer
		require.NoError(t, route.Render(context.Background(), params).Render(context.Background(), &out))
		return out.String()
	}
	t.Fatalf("no dock route named %q", name)
	return ""
}

// doRequest performs a request against the router and returns the status and the body.
func doRequest(t *testing.T, router http.Handler, method string, target string, form url.Values) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	request := httptest.NewRequest(method, target, body)
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	request.Header.Set("HX-Request", "true")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder, recorder.Body.String()
}

// seedEveryStage inserts one task per board stage in the workspace and returns their titles by stage.
func seedEveryStage(t *testing.T, repo *Repo, workspaceID string) map[string]string {
	t.Helper()
	statuses := map[string]models.TaskStatus{
		"backlog":   models.StatusBacklog,
		"working":   models.StatusRunning,
		"attention": models.StatusFailed,
		"review":    models.StatusAwaitingReview,
		"done":      models.StatusMerged,
	}
	titles := make(map[string]string, len(statuses))
	for stage, status := range statuses {
		title := "task-" + stage
		insertTask(t, repo, newTask(t, workspaceID, title, status))
		titles[stage] = title
	}
	return titles
}

func TestTabsOwnTheHomeTheEditTabTheWorkspaceAddressesTheDetailAndTheDiff(t *testing.T) {
	t.Parallel()
	module, _ := boardRouter(t, newRepo(t))

	var names []string
	for _, route := range module.Tabs() {
		names = append(names, route.Name+" "+route.Pattern)
	}

	assert.Equal(t, []string{"board /", "task_edit /tasks/{id}/edit", "board_workspace /board/{id}", "task_detail /tasks/{id}", "task_files /tasks/{id}/files", "task_file /tasks/{id}/files/{path...}", "task_diff /tasks/{id}/diff"}, names)
}

func TestBoardRendersEveryStageAndTheTaskLinks(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	titles := seedEveryStage(t, repo, workspaceID)
	module, _ := boardRouter(t, repo)

	page := renderTab(t, module, "board", dock.Params{})

	for stage, title := range titles {
		if stage == "done" {
			continue // completed tasks are hidden until the toggle is on
		}
		assert.Contains(t, page, title, "stage %s must show its task", stage)
		assert.Contains(t, page, `data-stage="`+stage+`"`, "stage %s must have a column", stage)
	}
	assert.Contains(t, page, `href="/tasks/`, "every card links to its task detail")
	assert.Contains(t, page, "To do")
	assert.Contains(t, page, "Needs attention")
	assert.NotContains(t, page, "task-done", "completed tasks stay hidden until the toggle is on")
}

func TestBoardShowsTheEmptyStateWithoutTasks(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)

	page := renderTab(t, module, "board", dock.Params{})

	assert.Contains(t, page, "No tasks in this workspace yet")
	assert.Contains(t, page, "Create the first task")
}

func TestBoardWithoutWorkspacesPointsToWorkspaces(t *testing.T) {
	t.Parallel()
	module, _ := boardRouter(t, newRepo(t))

	page := renderTab(t, module, "board", dock.Params{})

	assert.Contains(t, page, "Register a workspace to start planning tasks.")
	assert.Contains(t, page, `href="/workspaces"`)
}

func TestBoardOfAnUnknownWorkspaceSaysSo(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)

	page := renderTab(t, module, "board_workspace", dock.Params{"id": "not-a-workspace"})

	assert.Contains(t, page, "This workspace no longer exists.")
}

func TestBoardListFiltersByWorkspaceAndCompletedToggle(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	first := insertWorkspace(t, repo.DB, t.TempDir())
	second := insertWorkspace(t, repo.DB, t.TempDir())
	insertTask(t, repo, newTask(t, first, "in-first", models.StatusRunning))
	insertTask(t, repo, newTask(t, second, "in-second", models.StatusRunning))
	insertTask(t, repo, newTask(t, first, "merged-first", models.StatusMerged))
	_, router := boardRouter(t, repo)

	_, active := doRequest(t, router, http.MethodGet, "/board/list?workspace="+first, nil)
	_, completed := doRequest(t, router, http.MethodGet, "/board/list?workspace="+first+"&completed=1&pages=1", nil)

	assert.Contains(t, active, "in-first")
	assert.NotContains(t, active, "in-second", "another workspace's tasks never show")
	assert.NotContains(t, active, "merged-first", "completed tasks wait for the toggle")
	assert.Contains(t, completed, "merged-first")
	assert.NotContains(t, completed, "in-second")
}

// The list element listens to the task stream, so it stays in the page: a refresh swaps only its content. The
// endpoint /board/list answers that content alone; the element itself comes from the page or from /board/frame.
func TestBoardListEndpointAnswersOnlyTheListContent(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	insertTask(t, repo, newTask(t, workspaceID, "listed-only", models.StatusRunning))
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodGet, "/board/list?workspace="+workspaceID+"&completed=1&pages=2", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "listed-only", "the content of the list is answered")
	assert.NotContains(t, body, `id="board-list"`, "the listening element is not replaced by its own response")
	assert.NotContains(t, body, `hx-trigger=`, "the triggers stay on the element that stays in the page")
}

// The frame is the listening element with its address. The controls that change the address (the workspace, the
// completed toggle and the load more) swap it whole, so the next refresh repeats what the user chose.
func TestBoardFrameKeepsItsAddressForTheRefreshAndTheTaskStream(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodGet, "/board/frame?workspace="+workspaceID+"&completed=1&pages=2", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `id="board-list"`)
	assert.Contains(t, body, `hx-trigger="sse:changed, sse:removed, board:refresh from:body"`)
	assert.Contains(t, body, `hx-swap="innerHTML"`, "a refresh swaps the content and keeps the element")
	assert.Contains(t, body, `hx-sync="this:replace"`)
	assert.Contains(t, body, "/board/list?completed=1&amp;pages=2&amp;workspace="+workspaceID,
		"the refresh repeats the workspace and the completed toggle")
}

func TestBoardPageRendersTheStableListWithItsAddress(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)

	body := renderTab(t, module, "board_workspace", dock.Params{"id": workspaceID})

	assert.Contains(t, body, `id="board-list-`, "the page carries the list of its own instance")
	assert.Contains(t, body, `hx-trigger="sse:changed, sse:removed, board:refresh from:body"`)
	assert.Contains(t, body, `hx-get="/board/list?`, "the refresh asks for the content, not for the element")
	assert.Contains(t, body, `hx-swap="innerHTML"`)
	assert.Contains(t, body, `hx-get="/board/frame"`, "the toolbar swaps the whole element because it changes its address")
}

func TestBoardPageLeavesTheTaskStreamToTheShell(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)

	page := renderTab(t, module, "board", dock.Params{})

	assert.NotContains(t, page, `sse-connect=`, "the shell opens the one task stream of the page; a second one would receive every event twice")
}

func TestCreateTaskRoundTripRefreshesTheBoard(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/board/tasks", url.Values{
		"workspace_id": {workspaceID},
		"title":        {"Fix the login"},
		"prompt":       {"Investigate the failing login"},
		"priority":     {"high"},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"))
	assert.NotContains(t, body, `name="prompt"`, "a created task closes the dialog")
	assert.NotContains(t, body, `id="board-create-slot"`, "the response is swapped into the slot, so it must not repeat the slot id")
	assert.Contains(t, body, `id="board-notice" hx-swap-oob="true"`, "the notice slot is cleared out of band")
	_, list := doRequest(t, router, http.MethodGet, "/board/list?workspace="+workspaceID, nil)
	assert.Contains(t, list, "Fix the login", "the new task shows on the board")
}

func TestCreateTaskWithoutPromptKeepsTheDialogOpen(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/board/tasks", url.Values{
		"workspace_id": {workspaceID},
		"title":        {"No prompt"},
	})

	require.Equal(t, http.StatusOK, recorder.Code, "htmx swaps only 2xx responses: the error is the fragment")
	assert.Empty(t, recorder.Header().Get("HX-Trigger"))
	assert.Contains(t, body, "el prompt es obligatorio", "the domain error is visible in the dialog")
	assert.Contains(t, body, `name="prompt"`, "the dialog is shown again")
	assert.Contains(t, body, `value="No prompt"`, "the typed title is kept")
	_, list := doRequest(t, router, http.MethodGet, "/board/list?workspace="+workspaceID, nil)
	assert.NotContains(t, list, "No prompt", "nothing was created")
}

func TestNewTaskDialogSelectsTheWorkspaceTheBoardShows(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	_, body := doRequest(t, router, http.MethodGet, "/board/tasks/new?workspace="+workspaceID, nil)

	assert.Contains(t, body, `<option value="`+workspaceID+`" selected`)
	assert.Contains(t, body, "demo (main)", "the option names the workspace and its base branch")
}

func TestCloseNewTaskEmptiesTheDialogSlot(t *testing.T) {
	t.Parallel()
	_, router := boardRouter(t, newRepo(t))

	recorder, body := doRequest(t, router, http.MethodGet, "/board/tasks/new/close", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, body, `id="board-create-slot"`, "the slot is emptied by the swap, not refilled with its own id")
	assert.NotContains(t, body, `<form`, "the dialog is closed")
	assert.Contains(t, body, `id="board-notice" hx-swap-oob="true"`, "the notice slot is cleared out of band")
	assert.NotContains(t, body, `text-error`, "no notice without a failed start")
}

func TestSidebarLinksEachWorkspaceToItsBoardAndListsTheTasksThatNeedTheUser(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	failed := newTask(t, workspaceID, "needs-you", models.StatusFailed)
	insertTask(t, repo, failed)
	insertTask(t, repo, newTask(t, workspaceID, "quiet-work", models.StatusBacklog))
	module, _ := boardRouter(t, repo)

	var out bytes.Buffer
	require.NoError(t, module.Sidebar(context.Background()).Render(context.Background(), &out))
	sidebar := out.String()

	assert.Contains(t, sidebar, `id="board-sidebar"`)
	assert.Contains(t, sidebar, `hx-get="/board/sidebar"`)
	assert.Contains(t, sidebar, `hx-trigger="sse:changed, sse:removed, board:refresh from:body, htmx:afterRequest[`, "the sidebar refreshes on task changes and removals only, not on every stream event")
	assert.Contains(t, sidebar, `htmx:afterRequest[event.detail.pathInfo.requestPath.startsWith('/workspaces')]`, "a registration on the workspaces screen refreshes the sidebar")
	assert.Contains(t, sidebar, `hx-sync="this:replace"`, "a burst of stream events must end with a refetch that sees the last change, not be dropped while one is in flight")
	assert.Contains(t, sidebar, `hx-swap="innerHTML"`, "the listening element stays in the page and only its content is swapped")
	assert.Contains(t, sidebar, `href="/board/`+workspaceID+`"`)
	assert.Contains(t, sidebar, `href="/tasks/`+failed.ID.String()+`"`, "a failed task needs the user")
	assert.Contains(t, sidebar, "needs-you")
	assert.NotContains(t, sidebar, "quiet-work", "a backlog task does not need the user")
}

func TestSidebarEndpointAnswersItsOwnRefresh(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodGet, "/board/sidebar", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, body, `id="board-sidebar"`, "the listening element is not replaced by its own response")
	assert.Contains(t, body, `href="/board/`+workspaceID+`"`, "a workspace registered since the page loaded is listed")
}

func TestBoardListShowsTheLoadErrorInsteadOfTheEmptyState(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	breakTaskTable(t, repo)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodGet, "/board/list?workspace="+workspaceID, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `text-error`, "the failed load is shown as an error")
	assert.Contains(t, body, "no such table", "the reason of the failure is shown")
	assert.NotContains(t, body, "No tasks in this workspace yet", "a failed load is not an empty board")
}

func TestBoardTabShowsTheLoadErrorInsteadOfTheEmptyState(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	insertWorkspace(t, repo.DB, t.TempDir())
	breakTaskTable(t, repo)
	module, _ := boardRouter(t, repo)

	page := renderTab(t, module, "board", dock.Params{})

	assert.Contains(t, page, `text-error`)
	assert.NotContains(t, page, "No tasks in this workspace yet")
	assert.NotContains(t, page, "Create the first task", "the empty state offers a create that the broken board cannot back")
}

func TestBoardSaysWhenTheWorkspacesCannotBeReadInsteadOfNoneRegistered(t *testing.T) {
	t.Parallel()
	module, _ := boardRouterWith(t, newRepo(t), failingLookup{})

	page := renderTab(t, module, "board", dock.Params{})

	assert.Contains(t, page, "workspace registry unavailable", "the lookup failure is visible")
	assert.NotContains(t, page, "Register a workspace to start planning tasks.", "registered workspaces are not known to be missing")
}

func TestSidebarSaysWhenTheWorkspacesCannotBeReadAndKeepsRendering(t *testing.T) {
	t.Parallel()
	module, _ := boardRouterWith(t, newRepo(t), failingLookup{})

	var out bytes.Buffer
	require.NoError(t, module.Sidebar(context.Background()).Render(context.Background(), &out))
	sidebar := out.String()

	assert.Contains(t, sidebar, "workspace registry unavailable")
	assert.Contains(t, sidebar, `id="board-sidebar"`, "the sidebar still renders its sections")
}

func TestEditTabTitleFollowsTheLanguageOfTheRequest(t *testing.T) {
	t.Parallel()
	module, _ := boardRouter(t, newRepo(t))
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es})

	for _, route := range module.Tabs() {
		if route.Name == "task_edit" {
			assert.Equal(t, "Editar tarea", route.Title(ctx, dock.Params{"id": "x"}))
			return
		}
	}
	t.Fatal("no task_edit route")
}
