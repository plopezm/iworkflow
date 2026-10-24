//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"net/http"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/project_tree/presentation/components"
	"iworkflow/internal/modules/project_tree/presentation/state"
)

// serveChanges renders the body of the changes panel: the poll and the reload button.
func (h *Handlers) serveChanges(w http.ResponseWriter, r *http.Request) {
	target, err := dto.DecodeTarget(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	web.Render(w, r, http.StatusOK, components.GitChangesBody(h.gitView(r.Context(), target.Scope)))
}

// serveStage stages the posted paths (every path when none is posted) and renders the refreshed list.
func (h *Handlers) serveStage(w http.ResponseWriter, r *http.Request) {
	h.serveGitPaths(w, r, h.uc.StagePaths.Execute)
}

// serveUnstage removes the posted paths from the index and renders the refreshed list.
func (h *Handlers) serveUnstage(w http.ResponseWriter, r *http.Request) {
	h.serveGitPaths(w, r, h.uc.UnstagePaths.Execute)
}

// gitPathsAction is a stage or unstage use case.
type gitPathsAction func(ctx context.Context, scope models.ProjectScope, paths []string) error

// serveGitPaths runs one index action. The list is read again either way, so a failed action still shows
// what git holds.
func (h *Handlers) serveGitPaths(w http.ResponseWriter, r *http.Request, action gitPathsAction) {
	ctx := r.Context()
	form, err := dto.DecodePathsForm(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	actionErr := action(ctx, form.Scope, form.Paths)
	view := h.gitView(ctx, form.Scope)
	renderGitResult(w, r, view, actionErr, false)
}

// serveCommit commits what is staged. Only a successful commit clears the message; a failed one keeps it,
// so the user can fix the message and send it again.
func (h *Handlers) serveCommit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	form, err := dto.DecodeCommitForm(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	commitErr := h.uc.CommitChanges.Execute(ctx, form.Scope, form.Message)
	view := h.gitView(ctx, form.Scope)
	renderGitResult(w, r, view, commitErr, commitErr == nil)
}

// renderGitResult renders the list with the action outcome: the error slot, and the cleared message
// when cleared is set.
func renderGitResult(w http.ResponseWriter, r *http.Request, view state.GitView, actionErr error, cleared bool) {
	status := http.StatusOK
	if actionErr != nil {
		view.ActionError = actionErr.Error()
		status = errorStatus(actionErr)
	}
	web.Render(w, r, status, components.GitActionResponse(view, cleared))
}
