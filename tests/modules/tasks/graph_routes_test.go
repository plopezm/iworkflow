package tasks_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/tasks/domain/models"
)

func TestSetTaskPositionRoute(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "pos-task", models.StatusBacklog)
	insertTask(t, repo, task)
	module, router := boardRouter(t, repo)

	form := url.Values{"x": {"150.5"}, "y": {"280.0"}}
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+task.ID.String()+"/position", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)

	graph, err := module.GetGraph.Execute(context.Background(), workspaceID)
	require.NoError(t, err)
	require.Len(t, graph.Positions, 1)
	assert.Equal(t, task.ID, graph.Positions[0].TaskID)
	assert.Equal(t, 150.5, graph.Positions[0].Position.X)
	assert.Equal(t, 280.0, graph.Positions[0].Position.Y)
}

func TestAddTaskDependencyRoute(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	t1 := newTask(t, workspaceID, "pred", models.StatusRunning)
	t2 := newTask(t, workspaceID, "succ", models.StatusBacklog)
	insertTask(t, repo, t1)
	insertTask(t, repo, t2)
	module, router := boardRouter(t, repo)

	form := url.Values{"predecessor_id": {t1.ID.String()}}
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+t2.ID.String()+"/dependencies", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)

	graph, err := module.GetGraph.Execute(context.Background(), workspaceID)
	require.NoError(t, err)
	require.Len(t, graph.Dependencies, 1)
	assert.Equal(t, t1.ID, graph.Dependencies[0].Predecessor)
	assert.Equal(t, t2.ID, graph.Dependencies[0].Successor)
}

func TestRemoveTaskDependencyRoute(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	t1 := newTask(t, workspaceID, "pred", models.StatusRunning)
	t2 := newTask(t, workspaceID, "succ", models.StatusBacklog)
	insertTask(t, repo, t1)
	insertTask(t, repo, t2)
	module, router := boardRouter(t, repo)

	_, err := module.AddDependency.Execute(context.Background(), t1.ID, t2.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/tasks/"+t2.ID.String()+"/dependencies/"+t1.ID.String()+"/delete", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)

	graph, err := module.GetGraph.Execute(context.Background(), workspaceID)
	require.NoError(t, err)
	assert.Empty(t, graph.Dependencies)
}

func TestDetachTaskFromGraphRoute(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	t1 := newTask(t, workspaceID, "t1", models.StatusRunning)
	t2 := newTask(t, workspaceID, "t2", models.StatusBacklog)
	insertTask(t, repo, t1)
	insertTask(t, repo, t2)
	module, router := boardRouter(t, repo)

	_, err := module.AddDependency.Execute(context.Background(), t1.ID, t2.ID)
	require.NoError(t, err)
	err = module.SetPosition.Execute(context.Background(), t2.ID, 100.0, 200.0)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/tasks/"+t2.ID.String()+"/detach", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)

	graph, err := module.GetGraph.Execute(context.Background(), workspaceID)
	require.NoError(t, err)
	assert.Empty(t, graph.Dependencies)
	assert.Empty(t, graph.Positions)
}

func TestBoardGraphRouteRendersCanvas(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	t1 := newTask(t, workspaceID, "nodo-uno", models.StatusBacklog)
	insertTask(t, repo, t1)
	module, router := boardRouter(t, repo)

	err := module.SetPosition.Execute(context.Background(), t1.ID, 50.0, 60.0)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/board/graph?workspace="+workspaceID, nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, "graph-canvas")
	assert.Contains(t, body, "nodo-uno")
}

func TestBoardTabRendersGraphCanvasAndLoosePanel(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	t1 := newTask(t, workspaceID, "tarea-suelta", models.StatusBacklog)
	t2 := newTask(t, workspaceID, "tarea-grafo", models.StatusBacklog)
	insertTask(t, repo, t1)
	insertTask(t, repo, t2)
	module, router := boardRouter(t, repo)

	err := module.SetPosition.Execute(context.Background(), t2.ID, 100.0, 100.0)
	require.NoError(t, err)

	// Test 1: renderTab for dock route
	tabHtml := renderTab(t, module, "board_workspace", dock.Params{"id": workspaceID})
	assert.Contains(t, tabHtml, "board-graph")
	assert.Contains(t, tabHtml, "task-graph-canvas")
	assert.Contains(t, tabHtml, "Loose tasks")
	assert.Contains(t, tabHtml, "tarea-suelta")
	assert.Contains(t, tabHtml, "tarea-grafo")
	assert.Contains(t, tabHtml, "Fit view")

	// Test 2: HTTP frame endpoint for HTMX refresh
	res, body := doRequest(t, router, http.MethodGet, "/board/frame?workspace="+workspaceID, nil)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, body, "board-graph")
	assert.Contains(t, body, "task-graph-canvas")
	assert.Contains(t, body, "tarea-suelta")
	assert.Contains(t, body, "tarea-grafo")
}

func TestAddToGraphRoutePlacesNode(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	t1 := newTask(t, workspaceID, "suelta-para-anadir", models.StatusBacklog)
	insertTask(t, repo, t1)
	module, router := boardRouter(t, repo)

	req := httptest.NewRequest(http.MethodPost, "/board/tasks/"+t1.ID.String()+"/add-to-graph", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "board:refresh", res.Header().Get("HX-Trigger"))

	graph, err := module.GetGraph.Execute(context.Background(), workspaceID)
	require.NoError(t, err)
	require.Len(t, graph.Positions, 1)
	assert.Equal(t, t1.ID, graph.Positions[0].TaskID)
}

func TestBoardDetachRouteRemovesFromGraph(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	t1 := newTask(t, workspaceID, "nodo-para-quitar", models.StatusBacklog)
	insertTask(t, repo, t1)
	module, router := boardRouter(t, repo)

	err := module.SetPosition.Execute(context.Background(), t1.ID, 150.0, 150.0)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/board/tasks/"+t1.ID.String()+"/detach", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "board:refresh", res.Header().Get("HX-Trigger"))

	graph, err := module.GetGraph.Execute(context.Background(), workspaceID)
	require.NoError(t, err)
	assert.Empty(t, graph.Positions)
}
