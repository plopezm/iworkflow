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

// SubagentsReader lists the subagents a task reports. *application.GetTaskSubagents satisfies it.
type SubagentsReader interface {
	Execute(ctx context.Context, id models.TaskID) ([]models.SubagentTask, error)
}

// subagents answers the subagents panel of a task. The selected subagent is the one whose detail is open; a failed read
// is shown in the place of the panel.
func (h *DetailHandlers) subagents(w http.ResponseWriter, r *http.Request) {
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	list, err := h.uc.Subagents.Execute(r.Context(), id)
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	subagents := make([]dto.SubagentDTO, 0, len(list))
	for _, subagent := range list {
		subagents = append(subagents, dto.SubagentFromDomain(subagent))
	}
	view := state.NewSubagentsView(scope, id.String(), subagents, r.URL.Query().Get("selected"))
	web.Render(w, r, http.StatusOK, components.SubagentsPanel(view))
}
