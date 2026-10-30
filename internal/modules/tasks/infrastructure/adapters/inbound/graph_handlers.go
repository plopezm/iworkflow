//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// GraphUseCases are the application use cases needed to query and mutate the task graph.
type GraphUseCases struct {
	GetGraph         *application.GetTaskGraph
	AddDependency    *application.AddTaskDependency
	RemoveDependency *application.RemoveTaskDependency
	DetachFromGraph  *application.DetachTaskFromGraph
	SetPosition      *application.SetTaskPosition
	ListTasks        *application.ListTasks
	ListDoneTasks    *application.ListDoneTasks
}

// GraphHandlers serves HTTP endpoints for the dependency graph and node positions.
type GraphHandlers struct {
	uc  GraphUseCases
	now func() time.Time
}

// NewGraphHandlers constructs a new GraphHandlers.
func NewGraphHandlers(uc GraphUseCases, now func() time.Time) *GraphHandlers {
	return &GraphHandlers{uc: uc, now: now}
}

// Mount registers the routes on the router.
func (h *GraphHandlers) Mount(r chi.Router) {
	r.Get("/board/graph", h.boardGraph)
	r.Post("/tasks/{id}/position", h.setPosition)
	r.Post("/tasks/{id}/dependencies", h.addDependency)
	r.Post("/tasks/{id}/dependencies/{predecessor_id}/delete", h.removeDependencyByID)
	r.Delete("/tasks/dependencies", h.deleteDependencyByQuery)
	r.Post("/tasks/{id}/detach", h.detach)
}

// boardGraph serves the graph view (canvas + loose tasks panel) for a workspace.
func (h *GraphHandlers) boardGraph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspace")
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	showCompleted := r.URL.Query().Get("completed") == "1"

	graph, err := h.uc.GetGraph.Execute(ctx, workspaceID)
	if err != nil {
		slog.Error("failed to load task graph", "workspace", workspaceID, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	graphDTO := dto.TaskGraphFromDomain(graph)

	var wsPtr *string
	if workspaceID != "" {
		wsPtr = &workspaceID
	}
	tasks, err := h.uc.ListTasks.Execute(ctx, wsPtr)
	if err != nil {
		slog.Error("failed to load tasks for graph", "workspace", workspaceID, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	taskDTOs := make([]dto.TaskDTO, 0, len(tasks))
	now := h.now()
	for _, t := range tasks {
		taskDTOs = append(taskDTOs, dto.TaskFromDomain(t, now))
	}

	if showCompleted && h.uc.ListDoneTasks != nil {
		done, err := h.uc.ListDoneTasks.Execute(ctx, wsPtr, 0, state.DonePageSize)
		if err == nil {
			for _, t := range done {
				taskDTOs = append(taskDTOs, dto.TaskFromDomain(t, now))
			}
		}
	}

	graphView := state.BuildGraph(workspaceID, taskDTOs, graphDTO, showCompleted, nil)
	loose := state.LooseTasks(workspaceID, taskDTOs, graphDTO, showCompleted, nil)
	saved := state.SavedPositions(graphDTO)

	nodeIDs := make([]string, len(graphView.Nodes))
	for i, n := range graphView.Nodes {
		nodeIDs[i] = n.Task.ID
	}
	edges := make([][2]string, len(graphView.Edges))
	for i, e := range graphView.Edges {
		edges[i] = [2]string{e.PredecessorID, e.SuccessorID}
	}

	positions := state.ResolvePositions(nodeIDs, edges, saved)
	viewport := state.DefaultViewport()

	web.Render(w, r, http.StatusOK, components.BoardGraph(graphView, positions, viewport, loose, scope))
}

// setPosition records the canvas position (x, y) of a task node.
func (h *GraphHandlers) setPosition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskIDStr := chi.URLParam(r, "id")
	taskID, err := models.ParseTaskID(taskIDStr)
	if err != nil {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	x, err := strconv.ParseFloat(r.FormValue("x"), 64)
	if err != nil {
		http.Error(w, "invalid x coordinate", http.StatusBadRequest)
		return
	}
	y, err := strconv.ParseFloat(r.FormValue("y"), 64)
	if err != nil {
		http.Error(w, "invalid y coordinate", http.StatusBadRequest)
		return
	}

	if err := h.uc.SetPosition.Execute(ctx, taskID, x, y); err != nil {
		slog.Error("failed to set task position", "task_id", taskIDStr, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// addDependency connects two tasks: the task in the path waits for the predecessor.
func (h *GraphHandlers) addDependency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	succIDStr := chi.URLParam(r, "id")
	succID, err := models.ParseTaskID(succIDStr)
	if err != nil {
		http.Error(w, "invalid successor id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	predIDStr := r.FormValue("predecessor_id")
	predID, err := models.ParseTaskID(predIDStr)
	if err != nil {
		http.Error(w, "invalid predecessor id", http.StatusBadRequest)
		return
	}

	if _, err := h.uc.AddDependency.Execute(ctx, predID, succID); err != nil {
		slog.Warn("failed to add task dependency", "predecessor", predIDStr, "successor", succIDStr, "err", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("HX-Trigger", boardRefreshEvent)
	w.WriteHeader(http.StatusOK)
}

// removeDependencyByID removes an edge between two tasks.
func (h *GraphHandlers) removeDependencyByID(w http.ResponseWriter, r *http.Request) {
	succIDStr := chi.URLParam(r, "id")
	predIDStr := chi.URLParam(r, "predecessor_id")
	h.removeDependency(w, r, predIDStr, succIDStr)
}

// deleteDependencyByQuery removes an edge using query parameters.
func (h *GraphHandlers) deleteDependencyByQuery(w http.ResponseWriter, r *http.Request) {
	predIDStr := r.URL.Query().Get("predecessor")
	succIDStr := r.URL.Query().Get("successor")
	h.removeDependency(w, r, predIDStr, succIDStr)
}

func (h *GraphHandlers) removeDependency(w http.ResponseWriter, r *http.Request, predIDStr, succIDStr string) {
	ctx := r.Context()
	predID, err := models.ParseTaskID(predIDStr)
	if err != nil {
		http.Error(w, "invalid predecessor id", http.StatusBadRequest)
		return
	}
	succID, err := models.ParseTaskID(succIDStr)
	if err != nil {
		http.Error(w, "invalid successor id", http.StatusBadRequest)
		return
	}

	if err := h.uc.RemoveDependency.Execute(ctx, predID, succID); err != nil {
		slog.Error("failed to remove task dependency", "predecessor", predIDStr, "successor", succIDStr, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", boardRefreshEvent)
	w.WriteHeader(http.StatusOK)
}

// detach removes all dependencies and stored positions for a task.
func (h *GraphHandlers) detach(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskIDStr := chi.URLParam(r, "id")
	taskID, err := models.ParseTaskID(taskIDStr)
	if err != nil {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return
	}

	if err := h.uc.DetachFromGraph.Execute(ctx, taskID); err != nil {
		slog.Error("failed to detach task from graph", "task_id", taskIDStr, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", boardRefreshEvent)
	w.WriteHeader(http.StatusOK)
}
