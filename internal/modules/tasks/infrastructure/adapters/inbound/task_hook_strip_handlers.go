//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// msgHooksUnavailable answers a strip whose hooks cannot be read. The read error is logged, not shown: its text is internal.
var msgHooksUnavailable = i18n.Msg{
	En: "The hooks of the task could not be read.",
	Es: "No se pudieron leer los hooks de la tarea.",
	Zh: "无法读取任务的钩子。",
	De: "Die Hooks der Aufgabe konnten nicht gelesen werden.",
	Fr: "Les hooks de la tâche n’ont pas pu être lus.",
}

// msgHookSkipFailed answers a toggle the hooks refused. The error is logged, not shown: its text is internal.
var msgHookSkipFailed = i18n.Msg{
	En: "The hook could not be updated.",
	Es: "No se pudo actualizar el hook.",
	Zh: "无法更新钩子。",
	De: "Der Hook konnte nicht aktualisiert werden.",
	Fr: "Le hook n’a pas pu être mis à jour.",
}

// msgHookSkipInvalid answers a toggle that does not say whether to skip the hook.
var msgHookSkipInvalid = i18n.Msg{
	En: "The skip value is not valid.",
	Es: "El valor de omisión no es válido.",
	Zh: "跳过的值无效。",
	De: "Der Wert zum Überspringen ist ungültig.",
	Fr: "La valeur d’ignorance n’est pas valide.",
}

// hookStrip builds the status strip of a task from the hooks of its workspace. A read that fails is shown in the strip.
func (h *DetailHandlers) hookStrip(ctx context.Context, scope string, task models.Task) state.HookStrip {
	hooks, err := h.uc.HookStrip.State(ctx, task.WorkspaceID, task.ID.String())
	if err != nil {
		web.LogError(ctx, "could not read the hooks of the task", err, "task_id", task.ID.String())
		strip := state.NewHookStrip(scope, task.ID.String(), nil)
		strip.Error = msgHooksUnavailable.T(ctx)
		return strip
	}
	rows := make([]dto.HookStripDTO, 0, len(hooks))
	for _, hook := range hooks {
		rows = append(rows, dto.HookStripFromPort(hook))
	}
	return state.NewHookStrip(scope, task.ID.String(), rows)
}

// setHookSkipped sets the skip of one hook for the task and answers the strip of the task with the new state. A value
// that is not a boolean or a refused skip is shown in the strip, which keeps its rows.
func (h *DetailHandlers) setHookSkipped(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	if h.uc.HookStrip == nil {
		web.Render(w, r, http.StatusNotFound, shared.ErrorText(msgHooksUnavailable.T(ctx)))
		return
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if errors.Is(err, werrors.NotFound("")) {
		web.Render(w, r, http.StatusOK, components.TaskGone())
		return
	}
	if err != nil {
		web.LogError(ctx, "could not read the task of a hook toggle", err, "task_id", id.String())
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgTaskUnavailable.T(ctx)))
		return
	}
	failure := h.applyHookSkip(ctx, task, chi.URLParam(r, "hookId"), r)
	strip := h.hookStrip(ctx, scope, task)
	if failure != "" {
		strip.Error = failure
	}
	web.Render(w, r, http.StatusOK, components.TaskHookStrip(strip))
}

// applyHookSkip reads the wanted skip from the form and sets it. It returns the text to show when the skip was not
// set, and an empty text when it was.
func (h *DetailHandlers) applyHookSkip(ctx context.Context, task models.Task, hookID string, r *http.Request) string {
	if err := r.ParseForm(); err != nil {
		return msgHookSkipInvalid.T(ctx)
	}
	skipped, err := strconv.ParseBool(r.PostForm.Get("skipped"))
	if err != nil {
		return msgHookSkipInvalid.T(ctx)
	}
	if err := h.uc.HookStrip.SetSkipped(ctx, task.WorkspaceID, task.ID.String(), hookID, skipped); err != nil {
		web.LogError(ctx, "could not set the skip of a hook", err, "task_id", task.ID.String(), "hook_id", hookID)
		return msgHookSkipFailed.T(ctx)
	}
	return ""
}
