// Package inbound is the HTTP adapter of the workspaces module. It decodes the requests, calls the use
// cases and renders the tab pages and the htmx fragments. It holds no business logic.
package inbound

import "iworkflow/internal/modules/workspaces/application"

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
}
