//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"net/http"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/project_tree/presentation/components"
	"iworkflow/internal/modules/project_tree/presentation/state"
)

// serveFileBody renders the editor as the file is on disk: the reload button and the discard button.
func (h *Handlers) serveFileBody(w http.ResponseWriter, r *http.Request) {
	target, err := dto.DecodeTarget(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	view, err := h.fileView(r.Context(), target.Scope, target.Path, nil)
	if err != nil {
		renderError(w, r, err)
		return
	}
	web.Render(w, r, http.StatusOK, components.FileEditor(view))
}

// serveSaveFile writes the draft and renders the editor again. A failed save keeps the draft in the
// editor with the message, so nothing the user typed is lost, even when the file cannot be read back.
func (h *Handlers) serveSaveFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	form, err := dto.DecodeFileForm(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	saveErr := h.uc.WriteFile.Execute(ctx, form.Scope, form.Path, form.Content)
	view, readErr := h.fileView(ctx, form.Scope, form.Path, &form.Content)
	if readErr != nil {
		view = state.FileView{Scope: form.Scope, Path: form.Path, Draft: form.Content}
	}
	failure := saveErr
	if failure == nil {
		failure = readErr
	}
	if failure != nil {
		view.Error = failure.Error()
		web.Render(w, r, errorStatus(failure), components.FileEditor(view))
		return
	}
	web.Render(w, r, http.StatusOK, components.FileEditor(view))
}

// serveFileState answers each change of the draft with the toolbar and the line marks, out of band. The
// editor itself is not swapped, so the caret stays where the user types. An unreadable file has nothing
// to update: the response is empty.
func (h *Handlers) serveFileState(w http.ResponseWriter, r *http.Request) {
	form, err := dto.DecodeFileForm(r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	view, err := h.fileView(r.Context(), form.Scope, form.Path, &form.Content)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	web.Render(w, r, http.StatusOK, components.FileStateUpdate(view))
}
