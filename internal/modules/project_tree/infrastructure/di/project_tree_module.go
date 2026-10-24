// Package di is the composition root of the project_tree module: it builds the adapters, the use cases
// and the handlers, and exposes the routes and the dock tabs that the parent registers.
package di

import (
	"context"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/project_tree/application"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/outbound/fs"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/outbound/workspaceroot"
)

// Deps are the resources the module needs from the composition root. Both lookups are required.
type Deps struct {
	// Workspaces resolves workspace directories; the workspaces module provides it.
	Workspaces outbound.WorkspaceRootLookup
	// Tasks resolves task worktrees; the tasks module provides it.
	Tasks outbound.WorkspaceRootLookup
}

// Module exposes the use cases of the project_tree module, its routes and its dock tabs.
type Module struct {
	ListDirectory *application.ListProjectDirectory
	ReadFile      *application.ReadProjectFile
	WriteFile     *application.WriteProjectFile
	GitChanges    *application.ListProjectGitChanges
	FileBase      *application.ReadProjectFileBase
	StagePaths    *application.StageProjectPaths
	UnstagePaths  *application.UnstageProjectPaths
	CommitChanges *application.CommitProjectChanges
	SearchFiles   *application.SearchProjectFiles

	handlers *inbound.Handlers
}

// NewModule wires the adapters into the use cases and the use cases into the handlers.
func NewModule(deps Deps) *Module {
	roots := workspaceroot.NewResolverAdapter(deps.Workspaces, deps.Tasks)
	reader := fs.NewDirectoryReader()
	files := fs.NewFileStore()
	status := git.NewStatusAdapter()
	search := git.NewFileSearchAdapter()
	module := &Module{
		ListDirectory: application.NewListProjectDirectory(roots, reader),
		ReadFile:      application.NewReadProjectFile(roots, files),
		WriteFile:     application.NewWriteProjectFile(roots, files),
		GitChanges:    application.NewListProjectGitChanges(roots, status),
		FileBase:      application.NewReadProjectFileBase(roots, status),
		StagePaths:    application.NewStageProjectPaths(roots, status),
		UnstagePaths:  application.NewUnstageProjectPaths(roots, status),
		CommitChanges: application.NewCommitProjectChanges(roots, status),
		SearchFiles:   application.NewSearchProjectFiles(roots, search),
	}
	module.handlers = inbound.NewHandlers(inbound.UseCases{
		ListDirectory: module.ListDirectory,
		ReadFile:      module.ReadFile,
		WriteFile:     module.WriteFile,
		GitChanges:    module.GitChanges,
		FileBase:      module.FileBase,
		StagePaths:    module.StagePaths,
		UnstagePaths:  module.UnstagePaths,
		CommitChanges: module.CommitChanges,
		SearchFiles:   module.SearchFiles,
	})
	return module
}

// Routes mounts the fragment endpoints of the module. It satisfies web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Routes(r)
}

// Tabs returns the dock routes of the module: the project root of a workspace and its files. The
// composition root adds them to the dock registry.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{m.handlers.TreeTab(), m.handlers.FileTab()}
}

// RenderTree is the project root page for a wire scope. Other modules may embed it.
func (m *Module) RenderTree(ctx context.Context, scope string) templ.Component {
	return m.handlers.RenderTree(ctx, scope)
}

// RenderFile is the editor page of one file of a wire scope. Other modules may embed it.
func (m *Module) RenderFile(ctx context.Context, scope string, path string) templ.Component {
	return m.handlers.RenderFile(ctx, scope, path)
}

// RenderChanges is the changes panel of a scope, for the task views that embed it.
func (m *Module) RenderChanges(ctx context.Context, scope models.ProjectScope) templ.Component {
	return m.handlers.RenderChanges(ctx, scope)
}
