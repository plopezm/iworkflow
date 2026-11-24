//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/presentation/components"
	"iworkflow/internal/modules/workspace_groups/presentation/pages"
	"iworkflow/internal/modules/workspace_groups/presentation/state"
)

// WorkspaceInfo is a workspace as the group screens list it.
type WorkspaceInfo struct {
	ID    string
	Name  string
	Color string
}

// WorkspaceCatalog lists the workspaces whose groups are shown. It is consumer-side: the composition root implements it
// with the workspaces module, so this module never imports that one.
type WorkspaceCatalog interface {
	// List returns every workspace in the order the screens show them.
	List(ctx context.Context) ([]WorkspaceInfo, error)
}

// assignments reads the workspaces and places them under their groups.
func (h *Handlers) assignments(ctx context.Context) (models.Assignments, map[string]WorkspaceInfo, error) {
	workspaces, err := h.uc.Catalog.List(ctx)
	if err != nil {
		return models.Assignments{}, nil, fmt.Errorf("list the workspaces of the groups: %w", err)
	}
	ids := make([]string, 0, len(workspaces))
	byID := make(map[string]WorkspaceInfo, len(workspaces))
	for _, workspace := range workspaces {
		ids = append(ids, workspace.ID)
		byID[workspace.ID] = workspace
	}
	assigned, err := h.uc.Assignments.Execute(ctx, ids)
	if err != nil {
		return models.Assignments{}, nil, fmt.Errorf("list the groups: %w", err)
	}
	return assigned, byID, nil
}

// findGroup returns the members of one group, or ErrGroupNotFound.
func findGroup(assigned models.Assignments, id string) (models.GroupMembers, error) {
	for _, members := range assigned.Groups {
		if members.Group.ID() == id {
			return members, nil
		}
	}
	return models.GroupMembers{}, gerrors.ErrGroupNotFound
}

// ListTab renders the groups tab.
func (h *Handlers) ListTab(_ context.Context, _ dock.Params) templ.Component {
	return pages.GroupsPage(state.GroupsPage{})
}

// DetailTab renders the tab of one group. An unknown group shows its message.
func (h *Handlers) DetailTab(ctx context.Context, params dock.Params) templ.Component {
	assigned, _, err := h.assignments(ctx)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	members, err := findGroup(assigned, params["id"])
	if err != nil {
		return shared.ErrorText(pages.NotFound(ctx))
	}
	return pages.GroupPage(state.GroupPage{ID: members.Group.ID(), Name: members.Group.Name(), Color: members.Group.Color()})
}

// DetailTitle is the title of the tab of one group: its name, or the generic one when it cannot be read.
func (h *Handlers) DetailTitle(ctx context.Context, params dock.Params) string {
	assigned, _, err := h.assignments(ctx)
	if err == nil {
		if members, found := findGroup(assigned, params["id"]); found == nil {
			return members.Group.Name()
		}
	}
	return pages.ListTitle(ctx)
}

// list serves the rows of the groups tab: GET /workspace-groups/list.
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	assigned, _, err := h.assignments(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	rows := make([]state.GroupRow, 0, len(assigned.Groups))
	for _, members := range assigned.Groups {
		rows = append(rows, state.GroupRow{
			ID: members.Group.ID(), Name: members.Group.Name(), Color: members.Group.Color(),
			Count: len(members.Workspaces), Href: state.DetailPath(members.Group.ID()),
		})
	}
	h.render(w, r, components.GroupList(state.GroupList{Rows: rows}))
}

// members serves the workspaces of one group: GET /workspace-groups/{id}/members.
func (h *Handlers) members(w http.ResponseWriter, r *http.Request) {
	assigned, byID, err := h.assignments(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	group, err := findGroup(assigned, chi.URLParam(r, "id"))
	if err != nil {
		h.render(w, r, components.MembersGone())
		return
	}
	view := state.Members{GroupID: group.Group.ID(), Items: make([]state.Member, 0, len(group.Workspaces))}
	for _, id := range group.Workspaces {
		workspace := byID[id]
		view.Items = append(view.Items, state.Member{ID: workspace.ID, Name: workspace.Name, Color: workspace.Color})
	}
	h.render(w, r, components.Members(view))
}

// removeAndClose deletes a group from its own tab and answers the redirect to the groups list:
// POST /workspace-groups/{id}/delete-and-close.
func (h *Handlers) removeAndClose(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.Delete.Execute(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("HX-Trigger", RefreshEvent)
	h.render(w, r, pages.Deleted(state.ListPath))
}

func (h *Handlers) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		return
	}
}
