//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"errors"
	"net/http"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/web"
	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/project_tree/presentation/components"
	"iworkflow/internal/modules/project_tree/presentation/state"
)

// serveSearch lists the files that match the typed fragment, for the `@` references of the composer.
func (h *Handlers) serveSearch(w http.ResponseWriter, r *http.Request) {
	query, err := dto.DecodeSearchQuery(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	paths, err := h.uc.SearchFiles.Execute(r.Context(), query.Scope, query.Query)
	if err != nil {
		renderError(w, r, err)
		return
	}
	web.Render(w, r, http.StatusOK, components.SearchResults(paths))
}

// serveContentSearch renders the tree filtered by file content. An empty text restores the plain root
// listing, so clearing the box brings the tree back. A malformed regex is answered with 200 and the
// message in place of the tree, so htmx swaps it while the user is still typing.
func (h *Handlers) serveContentSearch(w http.ResponseWriter, r *http.Request) {
	scope, err := dto.ParseScope(r.FormValue("scope"))
	if err != nil {
		renderError(w, r, err)
		return
	}
	if r.FormValue("q") == "" {
		view, listErr := h.rootView(r.Context(), scope)
		web.Render(w, r, statusFor(listErr), components.TreeBody(view))
		return
	}
	request, err := dto.DecodeContentSearch(r)
	if errors.Is(err, treeerrors.InvalidSearch("")) {
		web.Render(w, r, http.StatusOK, coreui.ErrorText(err.Error()))
		return
	}
	if err != nil {
		renderError(w, r, err)
		return
	}
	result, err := h.uc.SearchContent.Execute(r.Context(), request.Scope, request.Query)
	if err != nil {
		renderError(w, r, err)
		return
	}
	web.Render(w, r, http.StatusOK, components.ContentResults(state.NewContentView(request.Scope, request.Query.Text(), result)))
}
