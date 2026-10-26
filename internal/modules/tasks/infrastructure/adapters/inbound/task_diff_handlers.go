//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// TaskDiffRouteName is the dock route name of the diff tab of a task.
const TaskDiffRouteName = "task_diff"

// msgDiffUnavailable answers a diff that cannot be read. The read error is logged, not shown: its text is internal.
var msgDiffUnavailable = i18n.Msg{
	En: "The diff of the task could not be read.",
	Es: "No se pudo leer el diff de la tarea.",
	Zh: "无法读取任务的差异。",
	De: "Der Diff der Aufgabe konnte nicht gelesen werden.",
	Fr: "Le diff de la tâche n’a pas pu être lu.",
}

// msgDiffTabPlain names the diff tab when the task cannot be read, so the title does not show a failure.
var msgDiffTabPlain = i18n.Msg{En: "Diff", Es: "Diff", Zh: "差异", De: "Diff", Fr: "Diff"}

// TaskGetter reads a task: the title of the tab and whether the task exists. The application use case satisfies it.
type TaskGetter interface {
	Execute(ctx context.Context, id models.TaskID) (models.Task, error)
}

// TaskDiffGetter reads the diff of a task against its base. *application.GetTaskDiff satisfies it.
type TaskDiffGetter interface {
	Execute(ctx context.Context, id models.TaskID) (models.TaskDiff, error)
}

// DiffUseCases are what the diff tab reads.
type DiffUseCases struct {
	Get  TaskGetter
	Diff TaskDiffGetter
}

// DiffHandlers serves the diff tab of a task and the view that its layout controls swap.
type DiffHandlers struct {
	uc DiffUseCases
}

// NewDiffHandlers builds the handlers over the use cases.
func NewDiffHandlers(uc DiffUseCases) *DiffHandlers {
	return &DiffHandlers{uc: uc}
}

// Tabs returns the dock route of the diff tab. The module registers it with the other tabs of the tasks.
func (h *DiffHandlers) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    TaskDiffRouteName,
			Pattern: "/tasks/{id}/diff",
			Icon:    "±",
			Title: func(ctx context.Context, params dock.Params) string {
				return h.title(ctx, params["id"])
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.tab(ctx, params["id"])
			},
		},
	}
}

// Mount registers the view of the diff. The layout controls answer it, with the layout in the query.
func (h *DiffHandlers) Mount(r chi.Router) {
	r.Get("/tasks/{id}/diff/view", h.view)
}

// title is the tab title: the title of the task, or the plain name when the task cannot be read.
func (h *DiffHandlers) title(ctx context.Context, rawID string) string {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return msgDiffTabPlain.T(ctx)
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil || task.Title == "" {
		return msgDiffTabPlain.T(ctx)
	}
	return pages.DiffTabTitle(ctx, task.Title)
}

// tab renders the diff tab of a task in the default layout. A task that no longer exists says so.
func (h *DiffHandlers) tab(ctx context.Context, rawID string) templ.Component {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return shared.ErrorText(msgInvalidTask.T(ctx))
	}
	if _, err := h.uc.Get.Execute(ctx, id); err != nil {
		return diffFailure(ctx, err)
	}
	view, err := h.load(ctx, id, newScope(), state.DefaultDiffLayout())
	if err != nil {
		return diffFailure(ctx, err)
	}
	return pages.TaskDiffPage(view)
}

// view answers the diff view in the layout of the query, for the instance named by its scope.
func (h *DiffHandlers) view(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidTask.T(ctx)))
		return
	}
	query := r.URL.Query()
	view, err := h.load(ctx, id, state.SanitizeScope(query.Get("scope")), state.ParseDiffLayout(query))
	if err != nil {
		web.Render(w, r, http.StatusOK, diffFailure(ctx, err))
		return
	}
	web.Render(w, r, http.StatusOK, components.DiffView(view))
}

// load reads the task and its diff, and builds the view of them. The files can be edited while the task has its worktree.
func (h *DiffHandlers) load(ctx context.Context, id models.TaskID, scope string, layout state.DiffLayout) (state.DiffView, error) {
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil {
		return state.DiffView{}, fmt.Errorf("read the task of the diff: %w", err)
	}
	diff, err := h.uc.Diff.Execute(ctx, id)
	if err != nil {
		return state.DiffView{}, fmt.Errorf("read the diff of the task: %w", err)
	}
	view := state.NewDiffView(scope, id.String(), dto.TaskDiffFromDomain(diff), layout)
	view.CanEdit = task.Status.HasWorktree()
	return view, nil
}

// diffFailure answers why a diff cannot be shown: a task that no longer exists says so, and any other failure is
// logged and answered in the request language.
func diffFailure(ctx context.Context, err error) templ.Component {
	if errors.Is(err, werrors.NotFound("")) {
		return components.TaskGone()
	}
	slog.Error("could not read the diff of the task", "error", err)
	return shared.ErrorText(msgDiffUnavailable.T(ctx))
}
