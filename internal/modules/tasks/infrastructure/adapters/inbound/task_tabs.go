//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Dock route names of the files of a task: the tree, and one of its files.
const (
	TaskFilesRouteName = "task_files"
	TaskFileRouteName  = "task_file"
)

// msgTaskFilesTitle names the Files dock tab of a task, copied from the Rust app (TITLE_FILES).
var msgTaskFilesTitle = i18n.Msg{En: "Files", Es: "Archivos", Zh: "文件", De: "Dateien", Fr: "Fichiers"}

// msgTaskUnavailable answers a tab whose task cannot be read. The read error is logged, not shown: its text is internal.
var msgTaskUnavailable = i18n.Msg{
	En: "The task could not be read.",
	Es: "No se pudo leer la tarea.",
	Zh: "无法读取任务。",
	De: "Die Aufgabe konnte nicht gelesen werden.",
	Fr: "La tâche n’a pas pu être lue.",
}

// msgPanelsUnavailable answers a tab whose project views were not injected. It is a wiring fault, not a user error.
var msgPanelsUnavailable = i18n.Msg{
	En: "The task files are not available.",
	Es: "Los ficheros de la tarea no están disponibles.",
	Zh: "任务文件暂不可用。",
	De: "Die Dateien der Aufgabe sind nicht verfügbar.",
	Fr: "Les fichiers de la tâche ne sont pas disponibles.",
}

// ProjectPanels are the project views and panels a task embeds: its file tree, the tree without its page, its changes,
// one of its files and its merge request panel. The composition root injects them, so tasks never imports project_tree
// or code_hosting.
type ProjectPanels interface {
	RenderTree(ctx context.Context, rawScope string) templ.Component
	TreePanel(ctx context.Context, rawScope string) templ.Component
	ChangesPanel(ctx context.Context, rawScope string) templ.Component
	RenderFile(ctx context.Context, rawScope string, file string) templ.Component
	MergeRequestPanel(ctx context.Context, taskID string) templ.Component
}

// fileRoutes are the dock routes of the files of a task. A file tab takes the name of its file.
func (h *DetailHandlers) fileRoutes() []dock.Route {
	return []dock.Route{
		{
			Name:    TaskFilesRouteName,
			Pattern: "/tasks/{id}/files",
			Icon:    "▤",
			Title: func(ctx context.Context, _ dock.Params) string {
				return msgTaskFilesTitle.T(ctx)
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.filesTab(ctx, params["id"])
			},
		},
		{
			Name:    TaskFileRouteName,
			Pattern: "/tasks/{id}/files/{path...}",
			Icon:    "◫",
			Title: func(_ context.Context, params dock.Params) string {
				return path.Base(params["path"])
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.fileTab(ctx, params["id"], params["path"])
			},
		},
	}
}

// filesTab is the page of the file tree of a task: the Files dock tab.
func (h *DetailHandlers) filesTab(ctx context.Context, rawID string) templ.Component {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	if h.uc.Panels == nil {
		return shared.ErrorText(msgPanelsUnavailable.T(ctx))
	}
	return h.uc.Panels.RenderTree(ctx, state.TaskPanelScope(id.String()))
}

// fileTab is the page of one file of a task, opened from its tree or its changes.
func (h *DetailHandlers) fileTab(ctx context.Context, rawID string, file string) templ.Component {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	if h.uc.Panels == nil {
		return shared.ErrorText(msgPanelsUnavailable.T(ctx))
	}
	return h.uc.Panels.RenderFile(ctx, state.TaskPanelScope(id.String()), file)
}

// detailPage is the detail page of a task on its Acciones tab: the header, the tabs and the transcript.
func (h *DetailHandlers) detailPage(ctx context.Context, scope string, task models.Task, notice string) templ.Component {
	view := h.page(ctx, scope, task, notice)
	tabs := state.TaskTabs{Scope: scope, TaskID: task.ID.String(), Active: state.TaskTabActions}
	return pages.TaskDetailPage(view, tabs, components.TaskActionsBody(view))
}

// tab answers the region of the tabs of a task with one tab active: the strip and the body of that tab. A tab the strip
// does not show is not found; a task that no longer exists says so.
func (h *DetailHandlers) tab(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tab, ok := state.ParseTaskTab(chi.URLParam(r, "tab"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if errors.Is(err, werrors.NotFound("")) {
		web.Render(w, r, http.StatusOK, components.TaskGone())
		return
	}
	if err != nil {
		slog.Error("could not read the task of a tab", "task", id.String(), "error", err)
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgTaskUnavailable.T(ctx)))
		return
	}
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	tabs := state.TaskTabs{Scope: scope, TaskID: id.String(), Active: tab}
	web.Render(w, r, http.StatusOK, components.TaskTabRegion(tabs, h.tabBody(ctx, scope, task, tab)))
}

// tabBody is the body of one tab of a task. Files and changes are the project views of the worktree of the task.
func (h *DetailHandlers) tabBody(ctx context.Context, scope string, task models.Task, tab string) templ.Component {
	if tab == state.TaskTabActions {
		return components.TaskActionsBody(h.page(ctx, scope, task, ""))
	}
	if h.uc.Panels == nil {
		return shared.ErrorText(msgPanelsUnavailable.T(ctx))
	}
	if tab == state.TaskTabCodeHost {
		return h.uc.Panels.MergeRequestPanel(ctx, task.ID.String())
	}
	wire := state.TaskPanelScope(task.ID.String())
	if tab == state.TaskTabChanges {
		return h.uc.Panels.ChangesPanel(ctx, wire)
	}
	return h.uc.Panels.TreePanel(ctx, wire)
}
