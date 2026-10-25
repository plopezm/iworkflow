//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// TaskDetailRouteName is the dock route name of the detail page of a task.
const TaskDetailRouteName = "task_detail"

// DetailUseCases are what the detail page reads and changes.
type DetailUseCases struct {
	Get    *application.GetTask
	Start  *application.StartTask
	Delete *application.DeleteTask
}

// DetailHandlers serves the detail tab of a task and its backlog actions.
type DetailHandlers struct {
	uc  DetailUseCases
	now func() time.Time
}

// NewDetailHandlers builds the handlers over the use cases. now is the clock the stage of the task is read at.
func NewDetailHandlers(uc DetailUseCases, now func() time.Time) *DetailHandlers {
	return &DetailHandlers{uc: uc, now: now}
}

// Tabs returns the dock route of the detail page. The module registers it before the not-migrated catch-all of
// /tasks/{rest...}, so only this address leaves the Rust version; diff, terminal and files still fall through.
func (h *DetailHandlers) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    TaskDetailRouteName,
			Pattern: "/tasks/{id}",
			Icon:    "●",
			Title: func(ctx context.Context, params dock.Params) string {
				return h.title(ctx, params["id"])
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.detailTab(ctx, params["id"])
			},
		},
	}
}

// Mount registers the htmx endpoints of the detail page: the start and the delete of a backlog task.
func (h *DetailHandlers) Mount(r chi.Router) {
	r.Post("/tasks/{id}/start", h.startTask)
	r.Post("/tasks/{id}/delete", h.deleteTask)
}

// title is the tab title: the title of the task, or the plain name when the task cannot be read.
func (h *DetailHandlers) title(ctx context.Context, rawID string) string {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return components.DetailTitle(ctx)
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil || task.Title == "" {
		return components.DetailTitle(ctx)
	}
	return task.Title
}

// detailTab renders the detail page of a task. An address that is not a task identifier shows the error; a task
// that no longer exists shows that it is gone, with a link back to the board.
func (h *DetailHandlers) detailTab(ctx context.Context, rawID string) templ.Component {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if errors.Is(err, werrors.NotFound("")) {
		return components.TaskGone()
	}
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	return pages.TaskDetailPage(state.TaskDetailView{Scope: newScope(), Task: dto.TaskFromDomain(task, h.now())})
}

// startTask starts a backlog task from its detail. The answer is the page again, with the failure of the start when
// there is one. The board refreshes either way, because a failed start may have put the task back in backlog.
func (h *DetailHandlers) startTask(w http.ResponseWriter, r *http.Request) {
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	_, startErr := h.uc.Start.Start(r.Context(), id)
	web.Trigger(w, boardRefreshEvent)
	h.renderPage(w, r, scope, id, errorText(startErr))
}

// deleteTask deletes a backlog task from its detail. A deleted task leaves the tab for its board; a refused delete
// keeps the tab and shows why in the result slot, where the delete button answers.
func (h *DetailHandlers) deleteTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	if deleteErr := h.uc.Delete.Execute(ctx, id); deleteErr != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(deleteErr.Error()))
		return
	}
	web.Trigger(w, boardRefreshEvent)
	web.Render(w, r, http.StatusOK, components.TaskSaved(state.BoardPath(task.WorkspaceID)))
}

// renderPage answers with the page of the task as it is now, carrying the notice of the action that asked for it.
func (h *DetailHandlers) renderPage(w http.ResponseWriter, r *http.Request, scope string, id models.TaskID, notice string) {
	task, err := h.uc.Get.Execute(r.Context(), id)
	if errors.Is(err, werrors.NotFound("")) {
		web.Render(w, r, http.StatusOK, components.TaskGone())
		return
	}
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	view := state.TaskDetailView{Scope: scope, Task: dto.TaskFromDomain(task, h.now()), Notice: notice}
	web.Render(w, r, http.StatusOK, pages.TaskDetailPage(view))
}
