// Package di is the composition root of the project_tree module: it builds the adapters and the use
// cases. Phase 1 exposes no routes or handlers; the presentation phase adds them.
package di

import (
	"iworkflow/internal/modules/project_tree/application"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
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

// Module exposes the use cases of the project_tree module.
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
}

// NewModule wires the adapters into the use cases.
func NewModule(deps Deps) *Module {
	roots := workspaceroot.NewResolverAdapter(deps.Workspaces, deps.Tasks)
	reader := fs.NewDirectoryReader()
	files := fs.NewFileStore()
	status := git.NewStatusAdapter()
	search := git.NewFileSearchAdapter()
	return &Module{
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
}
