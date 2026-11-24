package inbound

import (
	"context"
	"net/http"

	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/presentation/components"
	"iworkflow/internal/modules/workspace_groups/presentation/state"
)

// WorkspaceInfo is a workspace as the card lists it.
type WorkspaceInfo struct {
	ID    string
	Name  string
	Color string
}

// WorkspaceCatalog lists the workspaces to assign. It is consumer-side: the composition root implements it with the
// workspaces module, so this module never imports that one.
type WorkspaceCatalog interface {
	// List returns every workspace in the order the card shows them.
	List(ctx context.Context) ([]WorkspaceInfo, error)
}

// manage serves the group management card: GET /workspace-groups/manage[?open=<group id>].
func (h *Handlers) manage(w http.ResponseWriter, r *http.Request) {
	workspaces, err := h.uc.Catalog.List(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ids := make([]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		ids = append(ids, workspace.ID)
	}
	assignments, err := h.uc.Assignments.Execute(r.Context(), ids)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	view := buildManager(assignments, workspaces, r.URL.Query().Get("open"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.Manager(view).Render(r.Context(), w); err != nil {
		return
	}
}

// buildManager shapes the assignments for the card. Only the group named by open (when it exists) lists its members
// and the workspaces that can be assigned to it.
func buildManager(assignments models.Assignments, workspaces []WorkspaceInfo, open string) state.Manager {
	byID := make(map[string]WorkspaceInfo, len(workspaces))
	groupOf := make(map[string]string, len(workspaces))
	for _, workspace := range workspaces {
		byID[workspace.ID] = workspace
	}
	for _, members := range assignments.Groups {
		for _, id := range members.Workspaces {
			groupOf[id] = members.Group.Name()
		}
	}
	view := state.Manager{Groups: make([]state.ManagerGroup, 0, len(assignments.Groups))}
	for _, members := range assignments.Groups {
		entry := state.ManagerGroup{
			ID: members.Group.ID(), Name: members.Group.Name(), Color: members.Group.Color(),
			Count: len(members.Workspaces), Open: members.Group.ID() == open,
		}
		if entry.Open {
			view.OpenID = entry.ID
			inGroup := make(map[string]bool, len(members.Workspaces))
			for _, id := range members.Workspaces {
				inGroup[id] = true
				entry.Members = append(entry.Members, managerWorkspace(byID[id], ""))
			}
			for _, workspace := range workspaces {
				if !inGroup[workspace.ID] {
					entry.Available = append(entry.Available, managerWorkspace(workspace, groupOf[workspace.ID]))
				}
			}
		}
		view.Groups = append(view.Groups, entry)
	}
	return view
}

func managerWorkspace(workspace WorkspaceInfo, groupName string) state.ManagerWorkspace {
	return state.ManagerWorkspace{ID: workspace.ID, Name: workspace.Name, Color: workspace.Color, GroupName: groupName}
}
