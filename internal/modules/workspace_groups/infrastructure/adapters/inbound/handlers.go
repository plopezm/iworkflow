// Package inbound holds the HTTP adapter of the workspace_groups module. Every action answers 204 and asks
// the sidebar to refresh itself (HX-Trigger), so the server stays the source of truth for the layout.
package inbound

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/workspace_groups/application"
	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/inbound/dto"
)

// RefreshEvent is the event the sidebar listens to; it is the one the board already uses.
const RefreshEvent = "board:refresh"

// UseCases are the use cases the handlers call.
type UseCases struct {
	Create   *application.CreateGroup
	Update   *application.UpdateGroup
	Delete   *application.DeleteGroup
	Collapse *application.SetGroupCollapsed
	Move     *application.MoveWorkspace
	Pin      *application.SetPinned
	// Assignments and Catalog feed the group screens.
	Assignments *application.ListAssignments
	Catalog     WorkspaceCatalog
}

// Handlers serves the group actions.
type Handlers struct {
	uc UseCases
}

// NewHandlers builds the handlers over the use cases.
func NewHandlers(uc UseCases) *Handlers { return &Handlers{uc: uc} }

// Mount registers the endpoints.
//
//	POST /workspace-groups                            create a group (name, color; or an htmx prompt)
//	POST /workspace-groups/{id}                       rename and recolor a group
//	POST /workspace-groups/{id}/delete                delete a group, keeping its workspaces
//	POST /workspace-groups/{id}/collapse              collapsed=true|false
//	POST /workspace-groups/workspaces/{id}/group      group=<id>, or empty to ungroup
//	POST /workspace-groups/workspaces/{id}/pin        pinned=true|false
//	GET  /workspace-groups/list                       the rows of the groups tab
//	GET  /workspace-groups/{id}/members               the workspaces of a group
//	POST /workspace-groups/{id}/delete-and-close      delete a group from its own tab and go back to the list
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/workspace-groups/list", h.list)
	r.Get("/workspace-groups/{id}/members", h.members)
	r.Post("/workspace-groups", h.create)
	r.Post("/workspace-groups/workspaces/{id}/group", h.move)
	r.Post("/workspace-groups/workspaces/{id}/pin", h.pin)
	r.Post("/workspace-groups/{id}", h.update)
	r.Post("/workspace-groups/{id}/delete", h.remove)
	r.Post("/workspace-groups/{id}/delete-and-close", h.removeAndClose)
	r.Post("/workspace-groups/{id}/collapse", h.collapse)
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	form, err := dto.DecodeGroup(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	_, err = h.uc.Create.Execute(r.Context(), application.GroupInput{Name: form.Name, Color: form.Color})
	h.done(w, r, err)
}

func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	form, err := dto.DecodeGroup(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	_, err = h.uc.Update.Execute(r.Context(), chi.URLParam(r, "id"), application.GroupInput{Name: form.Name, Color: form.Color})
	h.done(w, r, err)
}

func (h *Handlers) remove(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.uc.Delete.Execute(r.Context(), chi.URLParam(r, "id")))
}

func (h *Handlers) collapse(w http.ResponseWriter, r *http.Request) {
	collapsed, err := dto.DecodeFlag(r, dto.FieldCollapsed)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.done(w, r, h.uc.Collapse.Execute(r.Context(), chi.URLParam(r, "id"), collapsed))
}

func (h *Handlers) move(w http.ResponseWriter, r *http.Request) {
	group, err := dto.DecodeField(r, dto.FieldGroup)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.done(w, r, h.uc.Move.Execute(r.Context(), chi.URLParam(r, "id"), group))
}

func (h *Handlers) pin(w http.ResponseWriter, r *http.Request) {
	pinned, err := dto.DecodeFlag(r, dto.FieldPinned)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.done(w, r, h.uc.Pin.Execute(r.Context(), chi.URLParam(r, "id"), pinned))
}

// done answers 204 with the sidebar refresh, or maps the error.
func (h *Handlers) done(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("HX-Trigger", RefreshEvent)
	w.WriteHeader(http.StatusNoContent)
}

// fail maps a domain error to a status: invalid input is 400, a missing group or workspace is 404 and
// anything else is a logged 500 with a generic message.
func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gerrors.ErrInvalidGroup):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, gerrors.ErrGroupNotFound), errors.Is(err, gerrors.ErrWorkspaceNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		web.LogError(r.Context(), "workspace group action failed", err)
		http.Error(w, "could not complete the action", http.StatusInternalServerError)
	}
}
