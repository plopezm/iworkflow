//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/components"
	"iworkflow/internal/modules/jira/presentation/state"
)

// epicParam reads and validates the epic key of the route; anything else is "no epic".
func epicParam(r *http.Request) string {
	key := strings.TrimSpace(chi.URLParam(r, "epic"))
	if key == models.NoEpicKey || models.IsValidIssueKey(key) {
		return key
	}
	return ""
}

// StatusMenu answers GET /jira/epics/{epic}/issues/{key}/transitions with the status changes Jira
// allows for one child issue. It is fetched on demand because every issue has its own workflow.
func (h *BoardHandlers) StatusMenu(w http.ResponseWriter, r *http.Request) {
	menu := state.StatusMenuView{EpicKey: epicParam(r), IssueKey: chi.URLParam(r, "key")}
	transitions, err := h.ListTransitions.Execute(r.Context(), menu.IssueKey)
	if err != nil {
		menu.Err = err.Error()
	} else {
		menu.Transitions = dto.TransitionsFromDomain(transitions)
	}
	web.Render(w, r, http.StatusOK, components.EpicStatusMenu(menu))
}

// ChangeStatus answers POST /jira/epics/{epic}/issues/{key}/transition: it applies the chosen
// transition in Jira and returns the whole board with the same epic open. A failure is shown as a
// notice above the board; the issue keeps its status.
func (h *BoardHandlers) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	var notice *string
	_, err := h.TransitionIssue.Execute(r.Context(), chi.URLParam(r, "key"), r.FormValue("transition_id"))
	if err != nil {
		message := err.Error()
		notice = &message
	}
	h.renderBoard(w, r, boardRenderOpts{
		filter:     extractFilter(r),
		laneNotice: notice,
		mode:       state.BoardModeEpics,
		epic:       epicParam(r),
	})
}
