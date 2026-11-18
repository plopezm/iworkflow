//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// FinishUseCase is what the finish action and its dialog use. *application.FinishTask satisfies it.
type FinishUseCase interface {
	Execute(ctx context.Context, id models.TaskID) error
	Preview(ctx context.Context, id models.TaskID) ([]models.WorktreeProcess, error)
	ExecuteClosing(ctx context.Context, id models.TaskID, selected []models.WorktreeProcess) error
}

// finishPreview answers the dialog that asks before a finish: its confirmation and, when processes use the worktree,
// one checked checkbox per process. A refusal or a failed read is shown in the place of the dialog.
func (h *DetailHandlers) finishPreview(w http.ResponseWriter, r *http.Request) {
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	processes, err := h.uc.Finish.Preview(r.Context(), id)
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	rows := make([]dto.FinishProcessDTO, 0, len(processes))
	for _, process := range processes {
		rows = append(rows, dto.FinishProcessFromDomain(process))
	}
	view := state.FinishPreviewView{Scope: scope, TaskID: id.String(), Processes: rows}
	web.Render(w, r, http.StatusOK, components.FinishPreview(view))
}
