// Package di is the composition root of the agents_files module: it builds the adapters and the
// use cases. The routes are added in a later phase; until then the module only exposes use cases.
package di

import (
	"database/sql"
	"log/slog"
	"time"

	"iworkflow/internal/modules/agents_files/application"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/fs"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/persistence"
)

// Deps are the resources the module needs from the composition root.
type Deps struct {
	DB *sql.DB
	// StorageDir is the folder where the AGENTS.md contents are stored.
	StorageDir string
	// Projects is provided by the workspaces module.
	Projects outbound.ProjectLookup
	Logger   *slog.Logger
}

// Module exposes the use cases of the agents_files module.
type Module struct {
	Upload       *application.UploadAgentsFile
	List         *application.ListAgentsFiles
	Delete       *application.DeleteAgentsFile
	Install      *application.InstallAgentsFile
	Apply        *application.ApplyAgentsFileToProject
	ListProjects *application.ListProjects
}

// NewModule wires the adapters into the use cases.
func NewModule(deps Deps) *Module {
	repository := persistence.NewAgentsFileRepository(deps.DB)
	storage := fs.NewAgentsFileStorage(deps.StorageDir)
	installer := git.NewAgentsFileInstaller(deps.Logger)
	clock := application.Clock(func() time.Time { return time.Now().UTC() })
	install := application.NewInstallAgentsFile(repository, storage, installer)
	return &Module{
		Upload:       application.NewUploadAgentsFile(repository, storage, clock, application.IDGenerator(models.NewAgentsFileID), deps.Logger),
		List:         application.NewListAgentsFiles(repository),
		Delete:       application.NewDeleteAgentsFile(repository, storage),
		Install:      install,
		Apply:        application.NewApplyAgentsFileToProject(deps.Projects, install),
		ListProjects: application.NewListProjects(deps.Projects),
	}
}
