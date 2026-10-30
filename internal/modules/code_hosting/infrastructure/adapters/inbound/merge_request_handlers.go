package inbound

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/code_hosting/presentation/components"
	"iworkflow/internal/modules/code_hosting/presentation/state"
)

// panelOutcome is what an action left to show in the panel of a task.
type panelOutcome struct {
	// err is the failure of the action; it replaces the state of the request in the panel.
	err error
	// notice confirms a successful action.
	notice state.Notice
	// confirming asks for the merge confirmation instead of the merge button.
	confirming bool
	// title keeps the title typed in the creation form.
	title string
}

// showPanel renders the panel of a task. ?confirm=1 shows the merge confirmation.
func (h *Handlers) showPanel(w http.ResponseWriter, r *http.Request) {
	confirming := r.URL.Query().Get("confirm") == "1"
	h.renderPanel(w, r, chi.URLParam(r, "id"), panelOutcome{confirming: confirming})
}

// createPanel opens the merge request of a task with the optional title of the form.
func (h *Handlers) createPanel(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	input := dto.CreateMergeRequestFromForm(taskID, r.PostForm).ToInput()
	_, err := h.createMergeRequest.Execute(r.Context(), input)
	h.renderPanel(w, r, taskID, panelOutcome{err: err, title: input.Title})
}

// mergePanel merges the open request of a task on the provider.
func (h *Handlers) mergePanel(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	_, err := h.mergeMergeRequest.Execute(r.Context(), taskID)
	h.renderPanel(w, r, taskID, panelOutcome{err: err})
}

// pushPanel publishes the branch of a task to origin.
func (h *Handlers) pushPanel(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	err := h.pushBranch.Execute(r.Context(), taskID)
	notice := state.NoNotice
	if err == nil {
		notice = state.NoticePushed
	}
	h.renderPanel(w, r, taskID, panelOutcome{err: err, notice: notice})
}

// renderPanel reads the request of the task again and renders the panel. Re-reading after every
// action keeps the panel in step with the provider. An action failure is shown instead of the state,
// as the Rust version did; a failed read leaves only the error.
func (h *Handlers) renderPanel(w http.ResponseWriter, r *http.Request, taskID string, outcome panelOutcome) {
	ctx := r.Context()
	data := state.PanelData{
		TaskID:     taskID,
		Title:      outcome.title,
		Notice:     outcome.notice,
		Confirming: outcome.confirming,
	}
	status, readErr := h.getMergeRequest.Execute(ctx, taskID)
	switch {
	case readErr != nil:
		data.Err = readErr.Error()
	case status == nil:
		data.NoRequest = true
	default:
		view := dto.MergeRequestStatusFrom(*status)
		data.Status = &view
	}
	if outcome.err != nil {
		data.Err = outcome.err.Error()
	}
	web.Render(w, r, http.StatusOK, components.TaskMergeRequestBody(state.MergeRequestPanel(ctx, data)))
}
