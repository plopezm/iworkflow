// Package di is the composition root of the code_hosting module: it builds the adapters and the use
// cases. Routes and presentation are added in the presentation phase.
package di

import (
	"database/sql"

	"iworkflow/internal/modules/code_hosting/application"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/api"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/persistence"
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

// Module exposes the use cases of the code_hosting module and the remote merge port.
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
}

// NewModule wires the adapters into the use cases.
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
	return &Module{
		GetSettings:            application.NewGetCodeHostSettings(repository),
		SaveSettings:           application.NewSaveCodeHostSettings(repository),
		TestConnection:         application.NewTestCodeHostConnection(repository, clients),
		GetTaskMergeRequest:    application.NewGetTaskMergeRequest(resolver),
		CreateTaskMergeRequest: application.NewCreateTaskMergeRequest(resolver, gitRemote),
		MergeTaskMergeRequest:  merge,
		PushTaskBranch:         application.NewPushTaskBranch(resolver, gitRemote),
		RemoteMerge:            remoteMerge,
	}
}
