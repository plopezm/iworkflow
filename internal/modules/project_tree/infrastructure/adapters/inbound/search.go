//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"net/http"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/project_tree/presentation/components"
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
