// Package di is the composition root of the code_hosting module: it builds the adapters, the use cases
// and the handlers, and exposes the routes and the components that other screens embed.
package di

import (
	"context"
	"database/sql"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/modules/code_hosting/application"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/api"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/code_hosting/presentation/components"
)

// Deps are the resources the module needs from the composition root.
type Deps struct {
	DB *sql.DB
	// Tasks resolves the repository, branch and prompt of a task. The tasks module provides it, and
	// the composition root adapts it.
	Tasks outbound.TaskSourceLookup
	// RemoteMerge is the port the tasks module consumes. It is created by the composition root before
	// both modules exist; NewModule binds it to the merge use case. Nil creates an internal one.
	RemoteMerge *application.RemoteMerge
}

// Module exposes the use cases of the code_hosting module, its remote merge port, its routes and the
// components the settings and task screens embed.
type Module struct {
	GetSettings            *application.GetCodeHostSettings
	SaveSettings           *application.SaveCodeHostSettings
	TestConnection         *application.TestCodeHostConnection
	GetTaskMergeRequest    *application.GetTaskMergeRequest
	CreateTaskMergeRequest *application.CreateTaskMergeRequest
	MergeTaskMergeRequest  *application.MergeTaskMergeRequest
	PushTaskBranch         *application.PushTaskBranch
	// RemoteMerge is bound to MergeTaskMergeRequest; the tasks module consumes it.
	RemoteMerge *application.RemoteMerge

	handlers *inbound.Handlers
}

// Routes mounts the fragment endpoints of the module. It satisfies web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// SettingsContent returns the settings card of a provider, for a settings section. The settings
// screen calls it on every render.
func (m *Module) SettingsContent(provider models.HostingProvider) func(context.Context) templ.Component {
	return m.handlers.SettingsContent(provider)
}

// TaskPanel returns the merge request panel of a task, for the task screen to embed. The panel loads
// its own content, so the screen passes only the task id.
func (m *Module) TaskPanel(taskID string) templ.Component {
	return components.TaskMergeRequestPanel(components.TaskMergeRequestPanelProps{TaskID: taskID})
}

// NewModule wires the adapters into the use cases and the use cases into the handlers.
func NewModule(deps Deps) *Module {
	repository := persistence.NewCodeHostSettingsRepository(deps.DB)
	clients := api.NewClientFactory(api.NewClientManager())
	gitRemote := git.NewRemoteAdapter()
	resolver := application.NewTaskProjectResolver(repository, deps.Tasks, gitRemote, clients)
	merge := application.NewMergeTaskMergeRequest(resolver, gitRemote)
	remoteMerge := deps.RemoteMerge
	if remoteMerge == nil {
		remoteMerge = application.NewRemoteMerge()
	}
	remoteMerge.Bind(merge)
	getSettings := application.NewGetCodeHostSettings(repository)
	saveSettings := application.NewSaveCodeHostSettings(repository)
	testConnection := application.NewTestCodeHostConnection(repository, clients)
	getMergeRequest := application.NewGetTaskMergeRequest(resolver)
	createMergeRequest := application.NewCreateTaskMergeRequest(resolver, gitRemote)
	pushTaskBranch := application.NewPushTaskBranch(resolver, gitRemote)
	return &Module{
		GetSettings:            getSettings,
		SaveSettings:           saveSettings,
		TestConnection:         testConnection,
		GetTaskMergeRequest:    getMergeRequest,
		CreateTaskMergeRequest: createMergeRequest,
		MergeTaskMergeRequest:  merge,
		PushTaskBranch:         pushTaskBranch,
		RemoteMerge:            remoteMerge,
		handlers: inbound.NewHandlers(
			getSettings,
			saveSettings,
			testConnection,
			getMergeRequest,
			createMergeRequest,
			merge,
			pushTaskBranch,
		),
	}
}
