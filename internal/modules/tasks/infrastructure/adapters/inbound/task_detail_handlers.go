//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"time"

	"github.com/a-h/templ"

	"iworkflow/internal/core/dock"
	shared "iworkflow/internal/core/components"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// TaskDetailRouteName is the dock route name of the detail page of a task.
const TaskDetailRouteName = "task_detail"

// DetailUseCases are what the detail page reads and changes.
type DetailUseCases struct {
	Get *application.GetTask
}

// DetailHandlers serves the detail tab of a task.
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
