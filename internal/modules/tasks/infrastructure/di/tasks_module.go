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
	httpinbound "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/events"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/files"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/storage"
)

// Deps are the resources the module needs from the composition root. Every field is required: the
// use cases call them unconditionally.
type Deps struct {
	DB *sql.DB
	// Workspaces reads the workspaces the tasks belong to; provided by the workspaces module.
	Workspaces outbound.WorkspaceLookup
	// AttachmentsDir is the folder the images of the tasks are stored in. The module creates it when missing.
	AttachmentsDir string
	// Git commits the changes saved from the diff view; the git port of slice E provides it.
	Git application.WorktreeCommitter
	// Worktrees tells where a task worktree goes (slice E).
	Worktrees application.WorktreeLayout
	// Start starts a backlog task; CreateTask uses it when a task is created with start_now (slices E and F).
	Start inbound.StartTaskUseCase
}

// Module exposes the use cases, the agent event sink the runtimes report to, the live event bus and
// the stream handlers. The event queue starts a worker goroutine: call ShutdownEvents when the
// application stops.
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
	// The attachments of a task: create, update, delete and the download of the stored images.
	ListAttachments *application.ListTaskAttachments
	GetAttachment   *application.GetTaskAttachment
	Cleanup         *application.AttachmentCleanup
	// Attachments serves the content of the stored images; the composition root mounts AttachmentRoute.
	Attachments *httpinbound.AttachmentHandler
	// The files of the worktrees, read and saved from the diff view.
	GetFile  *application.GetTaskFile
	SaveFile *application.SaveTaskFile
	// GetEvents reads the persisted history of a task after a cursor.
	GetEvents *application.GetTaskEvents
	// EventSink is what the agent runtimes report their events to: queued, so that a slow database
	// never blocks a runtime reader. Flush and ShutdownEvents belong to it.
	EventSink *application.QueuedAgentEventSink
	// Subscriber gives the live observers a subscription to the published events.
	Subscriber outbound.TaskEventSubscriber
	// Streams serves the task and board event streams; the composition root mounts them.
	Streams *httpinbound.TaskStreams
}

// NewModule wires the adapters into the use cases. It fails when a required dependency is missing,
// so that a partial composition cannot start.
func NewModule(deps Deps) (*Module, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}
	attachmentStorage, err := storage.NewFsAttachmentStorage(deps.AttachmentsDir)
	if err != nil {
		return nil, fmt.Errorf("tasks module: %w", err)
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
	// The bus is owned by the module: the lifecycle publishes to it and the streams subscribe to it.
	bus := events.NewEventBus(events.DefaultCapacity)
	lifecycle := application.NewTaskLifecycle(repository, bus, deps.Workspaces, clock)
	agentEvents := application.NewHandleAgentEvent(lifecycle, clock)
	getTask := application.NewGetTask(lifecycle)
	getEvents := application.NewGetTaskEvents(repository)
	attachments := persistence.NewTaskAttachmentRepository(deps.DB, persistence.Clock(clock))
	taskFiles := files.NewFsTaskFile()
	getAttachment := application.NewGetTaskAttachment(attachments, attachmentStorage)

	return &Module{
		Get:             getTask,
		List:            application.NewListTasks(repository),
		ListDone:        application.NewListDoneTasks(repository),
		Create:          application.NewCreateTask(lifecycle, deps.Worktrees, deps.Start, attachments, attachmentStorage, newUUID, clock),
		Update:          application.NewUpdateTask(lifecycle, attachments, attachmentStorage, newUUID),
		Delete:          application.NewDeleteTask(lifecycle, attachmentStorage),
		DeleteWorkspace: application.NewDeleteWorkspaceTasks(lifecycle, attachmentStorage),
		ListAttachments: application.NewListTaskAttachments(attachments),
		GetAttachment:   getAttachment,
		Cleanup:         application.NewAttachmentCleanup(attachments, attachmentStorage),
		Attachments:     httpinbound.NewAttachmentHandler(getAttachment),
		GetFile:         application.NewGetTaskFile(lifecycle, taskFiles),
		SaveFile:        application.NewSaveTaskFile(lifecycle, taskFiles, deps.Git),
		Lifecycle:       lifecycle,
		AgentEvents:     agentEvents,
		GetEvents:       getEvents,
		EventSink:       application.NewQueuedAgentEventSink(agentEvents, application.DefaultEventQueueCapacity),
		Subscriber:      bus,
		Streams:         httpinbound.NewTaskStreams(getTask, getEvents, bus),
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
		{"AttachmentsDir", d.AttachmentsDir == ""},
		{"Git", d.Git == nil},
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

// FlushEvents waits until every event the runtimes reported so far is processed and persisted.
func (m *Module) FlushEvents() {
	m.EventSink.Flush()
}

// ShutdownEvents stops the event queue after processing what it holds. Call it once, when the
// application stops; events reported afterwards are processed inline.
func (m *Module) ShutdownEvents() {
	m.EventSink.Shutdown()
}
