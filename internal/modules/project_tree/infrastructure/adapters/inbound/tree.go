//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"net/http"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/project_tree/presentation/components"
)

// serveTree renders the root listing: the reload button of the root tab.
func (h *Handlers) serveTree(w http.ResponseWriter, r *http.Request) {
	target, err := dto.DecodeTarget(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	view, listErr := h.rootView(r.Context(), target.Scope)
	web.Render(w, r, statusFor(listErr), components.TreeBody(view))
}

// serveDirectory renders the children of a folder when the user expands it.
func (h *Handlers) serveDirectory(w http.ResponseWriter, r *http.Request) {
	target, err := dto.DecodeTarget(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	view, listErr := h.childrenView(r.Context(), target.Scope, target.Path)
	web.Render(w, r, statusFor(listErr), components.TreeChildren(view))
}
