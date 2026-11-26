// Package inbound is the HTTP adapter of the workspace_branches module. It decodes the requests, calls the use
// cases and renders the branch tab and its htmx fragments. It holds no business logic.
//
//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/workspace_branches/application"
	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	"iworkflow/internal/modules/workspace_branches/domain/models"
	"iworkflow/internal/modules/workspace_branches/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/workspace_branches/presentation/components"
	"iworkflow/internal/modules/workspace_branches/presentation/state"
)

// fragmentStatus is the status of every htmx response, errors included: htmx 2 does not swap 4xx or 5xx
// responses by default, so the message travels as the fragment itself (see the workspaces module).
const fragmentStatus = http.StatusOK

// listUseCase and deleteUseCase are the parts of the use cases the handlers drive.
type listUseCase interface {
	Execute(ctx context.Context, workspaceID string, filter string) (application.BranchList, error)
}

type deleteUseCase interface {
	Execute(ctx context.Context, workspaceID string, refs []models.Ref) ([]application.DeleteResult, error)
}

// Handlers serves the branch tab and its htmx requests.
type Handlers struct {
	list   listUseCase
	delete deleteUseCase
}

// NewHandlers builds the handlers over the use cases.
func NewHandlers(list listUseCase, deleteBranches deleteUseCase) *Handlers {
	return &Handlers{list: list, delete: deleteBranches}
}

// Mount registers the htmx endpoints. The tab itself is served by the dock, see ListTab.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/workspaces/{id}/branches", h.listFragment)
	r.Post("/workspaces/{id}/branches/delete", h.deleteBranches)
}

// ListTab renders the branch tab of one workspace.
func (h *Handlers) ListTab(ctx context.Context, params dock.Params) templ.Component {
	view, err := h.screen(ctx, params["id"], "", nil, false)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	return components.BranchesScreen(view)
}

// listFragment answers the filter box with the list only. Without htmx it answers with the whole tab.
func (h *Handlers) listFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	filter := r.URL.Query().Get("q")
	view, err := h.screen(ctx, id, filter, nil, false)
	if !web.IsHTMX(r) {
		if err != nil {
			writeFragments(w, r, shared.ErrorText(err.Error()))
			return
		}
		writeFragments(w, r, components.BranchesScreen(view))
		return
	}
	if err != nil {
		writeFragments(w, r, components.BranchListBody(errorList(id, filter, err)))
		return
	}
	writeFragments(w, r, components.BranchListBody(view.List))
}

// deleteBranches deletes the checked branches (repeated "branch" values) or the one named in the query of a
// per-row request, and answers with the refreshed list and the outcome of each reference.
func (h *Handlers) deleteBranches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeFragments(w, r, components.BranchListBody(errorList(id, "", err)))
		return
	}
	form := dto.DeleteFormFrom(r.Form)
	noneSelected := len(form.Refs) == 0
	var outcomes []application.DeleteResult
	if !noneSelected {
		var err error
		outcomes, err = h.delete.Execute(ctx, id, form.Refs)
		if err != nil {
			writeFragments(w, r, components.BranchListBody(errorList(id, form.Query, err)))
			return
		}
	}
	view, err := h.screen(ctx, id, form.Query, outcomes, noneSelected)
	if err != nil {
		// The deletions already happened: keep their outcomes next to the refresh failure.
		failed := errorList(id, form.Query, err)
		failed.Results = resultLines(outcomes)
		writeFragments(w, r, components.BranchListBody(failed))
		return
	}
	writeFragments(w, r, components.BranchListBody(view.List))
}

// screen builds the tab view from the branch list. outcomes are the results of the delete request that produced it.
func (h *Handlers) screen(ctx context.Context, id string, filter string, outcomes []application.DeleteResult, noneSelected bool) (state.ScreenView, error) {
	list, err := h.list.Execute(ctx, id, filter)
	if err != nil {
		// The text of these errors is shown to the user as it is, like the workspaces module's.
		return state.ScreenView{}, err //nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible.
	}
	return state.ScreenView{
		WorkspaceName: list.Workspace.Name,
		BoardHref:     "/board/" + id,
		SearchHref:    "/workspaces/" + id + "/branches",
		List: state.ListView{
			Query:        list.Filter,
			DeleteAction: deleteAction(id),
			Rows:         rows(id, list),
			Results:      resultLines(outcomes),
			NoneSelected: noneSelected,
		},
	}, nil
}

// errorList is the list shown when a request failed: no rows, just the message.
func errorList(id string, filter string, err error) state.ListView {
	return state.ListView{Query: filter, DeleteAction: deleteAction(id), Error: errorText(err)}
}

func deleteAction(id string) string {
	return "/workspaces/" + id + "/branches/delete"
}

// rows maps the branches to the view rows. A protected branch keeps its reason and gets no working delete.
func rows(id string, list application.BranchList) []state.BranchRow {
	out := make([]state.BranchRow, 0, len(list.Branches))
	for _, branch := range list.Branches {
		ref := branch.Ref()
		out = append(out, state.BranchRow{
			Value:      ref.String(),
			Name:       branch.FullName(),
			Remote:     branch.Remote,
			Protected:  branch.Protected,
			Reason:     string(branch.Reason),
			DeleteHref: deleteAction(id) + "?" + url.Values{"branch": {ref.String()}, "q": {list.Filter}}.Encode(),
		})
	}
	return out
}

// resultLines maps the outcome of each reference of a delete request to a message line.
func resultLines(outcomes []application.DeleteResult) []state.ResultLine {
	out := make([]state.ResultLine, 0, len(outcomes))
	for _, outcome := range outcomes {
		line := state.ResultLine{Name: outcome.Ref.Name}
		switch outcome.Outcome {
		case application.OutcomeDeleted:
			line.Outcome = "deleted"
		case application.OutcomeRefused:
			line.Outcome = "refused"
			line.Reason = string(outcome.Reason)
		default:
			line.Outcome = "failed"
			line.Failure = failureKind(outcome.Err)
			if line.Failure == "git" && outcome.Err != nil {
				line.Error = outcome.Err.Error()
			}
		}
		out = append(out, line)
	}
	return out
}

func failureKind(err error) string {
	switch {
	case errors.Is(err, werrors.ErrInvalidBranch):
		return "invalid"
	case errors.Is(err, werrors.ErrBranchNotFound):
		return "not_found"
	default:
		return "git"
	}
}

func errorText(err error) string {
	return err.Error()
}

// writeFragments renders the components one after another as a single htmx response.
func writeFragments(w http.ResponseWriter, r *http.Request, fragments ...templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(fragmentStatus)
	for _, fragment := range fragments {
		if err := fragment.Render(r.Context(), w); err != nil {
			return
		}
	}
}
