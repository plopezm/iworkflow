// Package inbound is the HTTP adapter of the workspaces module. It decodes the requests, calls the use
// cases and renders the tab pages and the htmx fragments. It holds no business logic.
package inbound

import (
	"context"

	"iworkflow/internal/modules/workspaces/application"
)

// GroupInfo is a workspace group as the workspaces screens show it.
type GroupInfo struct {
	ID    string
	Name  string
	Color string
}

// GroupDirectory tells which group each workspace is in. It is consumer-side: the composition root implements it with
// the workspace_groups module, so this module never imports that one. Groups are managed on their own screens.
type GroupDirectory interface {
	// Directory returns every group in order, and the group id of each grouped workspace (by workspace id).
	Directory(ctx context.Context) ([]GroupInfo, map[string]string, error)
}

// UseCases are the use cases the adapter drives. The composition root builds them.
type UseCases struct {
	Register        *application.RegisterWorkspace
	List            *application.ListWorkspaces
	Get             *application.GetWorkspace
	Update          *application.UpdateWorkspace
	Delete          *application.DeleteWorkspace
	BrowseDirectory *application.BrowseDirectories
	CreateDirectory *application.CreateDirectory
	ListAgentsFiles *application.ListAgentsFileOptions
	// ListAgentRuntimes lists the agents a workspace can take as its default.
	ListAgentRuntimes *application.ListAgentRuntimes
	// ListWithMemory lists the workspaces of the table with their memory summary.
	ListWithMemory *application.ListWorkspacesWithMemory
	// Groups tells the group of each workspace. Optional: nil hides the group chip and the group selector.
	Groups GroupDirectory
}
