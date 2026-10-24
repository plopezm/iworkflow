// Package di is the composition root of the tasks module. It builds the adapters and the slice A use
// cases from the resources the composition root provides. Handlers, the other slices and the routes
// join this module as their slices are migrated (see SLICES.md).
package di

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence"
)

// Deps are the resources the module needs from the composition root. Every field is required: the
// use cases call them unconditionally.
type Deps struct {
	DB *sql.DB
	// Workspaces reads the workspaces the tasks belong to; provided by the workspaces module.
	Workspaces outbound.WorkspaceLookup
	// Publisher broadcasts task events live; provided by the event bus (slice B).
	Publisher outbound.TaskEventPublisher
	// Attachments stores the image metadata and Storage the image files (slice D).
	Attachments outbound.TaskAttachmentRepository
	Storage     outbound.AttachmentStorage
	// Worktrees tells where a task worktree goes (slice E).
	Worktrees application.WorktreeLayout
	// Start starts a backlog task; CreateTask uses it when a task is created with start_now (slices E and F).
	Start inbound.StartTaskUseCase
}

// Module exposes the slice A use cases and the agent event sink.
type Module struct {
	Get             *application.GetTask
	List            *application.ListTasks
	ListDone        *application.ListDoneTasks
	Create          *application.CreateTask
	Update          *application.UpdateTask
	Delete          *application.DeleteTask
	AnswerDialog    *application.AnswerDialog
	GetSubagents    *application.GetTaskSubagents
	DeleteWorkspace *application.DeleteWorkspaceTasks
	AgentEvents     *application.HandleAgentEvent
	Lifecycle       *application.TaskLifecycle
}

// NewModule wires the adapters into the use cases. It fails when a required dependency is missing,
// so that a partial composition cannot start.
func NewModule(deps Deps) (*Module, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}
	clock := application.Clock(func() time.Time { return time.Now().UTC() })
	newUUID := application.UUIDGenerator(uuid.NewRandom)

	// The main repository retries transient (busy) errors with the bounded policy, as the Rust version does.
	repository := persistence.NewRetryingTaskRepository(
		persistence.NewTaskRepository(deps.DB, persistence.Clock(clock)),
		persistence.BoundedRetryPolicy(),
		persistence.Clock(clock),
		persistence.ContextSleep,
	)
	lifecycle := application.NewTaskLifecycle(repository, deps.Publisher, deps.Workspaces, clock)

	return &Module{
		Get:             application.NewGetTask(lifecycle),
		List:            application.NewListTasks(repository),
		ListDone:        application.NewListDoneTasks(repository),
		Create:          application.NewCreateTask(lifecycle, deps.Worktrees, deps.Start, deps.Attachments, deps.Storage, newUUID, clock),
		Update:          application.NewUpdateTask(lifecycle, deps.Attachments, deps.Storage, newUUID),
		Delete:          application.NewDeleteTask(lifecycle, deps.Storage),
		DeleteWorkspace: application.NewDeleteWorkspaceTasks(lifecycle, deps.Storage),
		Lifecycle:       lifecycle,
		AgentEvents:     application.NewHandleAgentEvent(lifecycle, clock),
	}, nil
}

// errMissingDependency is returned when a required dependency of the module is nil.
var errMissingDependency = errors.New("tasks module: missing dependency")

// validate reports the first required dependency that is nil.
func (d Deps) validate() error {
	required := []struct {
		name    string
		missing bool
	}{
		{"DB", d.DB == nil},
		{"Workspaces", d.Workspaces == nil},
		{"Publisher", d.Publisher == nil},
		{"Attachments", d.Attachments == nil},
		{"Storage", d.Storage == nil},
		{"Worktrees", d.Worktrees == nil},
		{"Start", d.Start == nil},
	}
	for _, dependency := range required {
		if dependency.missing {
			return fmt.Errorf("%w: %s", errMissingDependency, dependency.name)
		}
	}
	return nil
}
