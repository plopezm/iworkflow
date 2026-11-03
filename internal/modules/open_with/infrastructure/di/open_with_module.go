// Package di composes the open_with module with constructor injection.
package di

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/open_with/application"
	"iworkflow/internal/modules/open_with/domain/ports/outbound"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/outbound/filesystem"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/outbound/process"
)

// Deps are the resources the module is built from.
type Deps struct {
	DB *sql.DB
	// Worktrees resolves the worktree of a task; the composition root adapts the tasks module.
	Worktrees outbound.WorktreeLookup
}

// Module exposes the use cases and mounts the dialog endpoints.
type Module struct {
	List   *application.ListLaunchers
	Create *application.CreateLauncher
	Delete *application.DeleteLauncher
	Launch *application.Launch
	Browse *application.BrowseFiles
	Pick   *application.PickExecutable

	handlers *inbound.Handlers
}

var _ web.Module = (*Module)(nil)

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.NewString() }

// NewModule builds the adapters, the use cases and the handlers.
func NewModule(deps Deps) *Module {
	repository := persistence.NewLauncherRepository(deps.DB)
	module := &Module{
		List:   application.NewListLaunchers(repository),
		Create: application.NewCreateLauncher(repository, uuidGenerator{}),
		Delete: application.NewDeleteLauncher(repository),
		Launch: application.NewLaunch(repository, deps.Worktrees, process.NewStarter()),
		Browse: application.NewBrowseFiles(filesystem.NewBrowser()),
		Pick:   application.NewPickExecutable(filesystem.NewNativePicker()),
	}
	module.handlers = inbound.NewHandlers(inbound.UseCases{
		List: module.List, Create: module.Create, Delete: module.Delete, Launch: module.Launch, Browse: module.Browse, Pick: module.Pick,
	})
	return module
}

// Routes mounts the dialog endpoints.
func (m *Module) Routes(r chi.Router) { m.handlers.Mount(r) }
