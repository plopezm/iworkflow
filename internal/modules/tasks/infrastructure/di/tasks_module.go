// Package di is the composition root of the tasks module. It builds the adapters and the use cases from the
// resources the composition root provides, and mounts the routes the module owns (see SLICES.md).
package di

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	httpinbound "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/events"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/files"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/git"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime/claude"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime/cursor"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime/pi"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/storage"
)

// Runtime identifiers. They are the values the agent settings store, and the gentle-shell runtime is the default.
const (
	runtimeGentleShell = "gentle-shell"
	runtimePi          = "pi"
	runtimeClaude      = "claude"
	runtimeCursor      = "cursor"
)

// RuntimeBinaries are the executables and the sessions folder the agent runtimes launch with. The composition
// root reads them from the configuration; every field is required.
type RuntimeBinaries struct {
	// GentleShell is the gentle-shell executable launched in RPC mode (the default runtime).
	GentleShell string
	// Pi is the pi executable launched in RPC mode.
	Pi string
	// Claude is the Claude Code executable launched in stream-json mode.
	Claude string
	// Cursor is the Cursor agent executable, launched as `agent acp`.
	Cursor string
	// CursorSessionsDir stores each task's Cursor session id, so that a resume continues it.
	CursorSessionsDir string
}

// Deps are the resources the module needs from the composition root. The fields marked optional may be
// nil; every other field is required, because the use cases call it unconditionally.
type Deps struct {
	DB *sql.DB
	// Workspaces reads the workspaces the tasks belong to; provided by the workspaces module.
	Workspaces outbound.WorkspaceLookup
	// AttachmentsDir is the folder the images of the tasks are stored in. The module creates it when missing.
	AttachmentsDir string
	// WorktreesDir is the root of the task worktrees. Every worktree the module creates or removes lives
	// under it. The module creates the git adapter over it.
	WorktreesDir string
	// Binaries configure the agent runtimes. The module builds them, as the Rust tasks context does.
	Binaries RuntimeBinaries
	// LaunchSettings prepares the launch options of every start and resume from the agent settings.
	LaunchSettings outbound.LaunchSettings
	// Remote merges a task through its open merge request in the provider (code_hosting). Optional: nil
	// means every task is merged locally.
	Remote outbound.RemoteMerge
	// FileSearch lists the project files of a task for the @ completion of the composer. Optional: nil means
	// the completion finds nothing. The composition root supplies it, so the tasks module does not import the
	// module that searches the files.
	FileSearch httpinbound.FileSearch
	// Panels are the project views (tree, changes, file) the Files and Changes tabs of a task embed. Optional: nil
	// makes those tabs answer that they are not available.
	Panels httpinbound.ProjectPanels
}

// Module exposes the use cases, the agent event sink the runtimes report to, the live event bus and
// the stream handlers. The event queue starts a worker goroutine: call ShutdownEvents when the
// application stops. The agents the module starts are stopped with StopAgents.
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
	// The agent life cycle of a task: start, messages, mode, abort and the terminal actions. Finish, reset and
	// discard stop the agent of the task and remove its worktree.
	Start        *application.StartTask
	Send         *application.SendMessage
	SetAgentMode *application.SetAgentMode
	Abort        *application.AbortTask
	Finish       *application.FinishTask
	Reset        *application.ResetTask
	Discard      *application.DiscardTask
	// Reconcile marks the tasks a restart interrupted; the composition root runs it once at start-up.
	Reconcile *application.ReconcileTasks
	// The git side: the worktree port (cached diffs) and the use cases over it.
	Worktrees      outbound.Worktree
	GetDiff        *application.GetTaskDiff
	Merge          *application.MergeTask
	SweepWorktrees *application.SweepOrphanWorktrees
	ListBranches   *application.ListWorkspaceBranches
	// The attachments of a task: create, update, delete and the download of the stored images.
	ListAttachments *application.ListTaskAttachments
	GetAttachment   *application.GetTaskAttachment
	Cleanup         *application.AttachmentCleanup
	// Attachments serves the content of the stored images. Routes mounts it.
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
	// Streams serves the task and board event streams. Routes mounts them.
	Streams *httpinbound.TaskStreams
	// Board serves the board tabs (Tabs) and the htmx endpoints of the list and the create dialog (Routes).
	Board *httpinbound.BoardHandlers
	// Detail serves the detail tab of a task (Tabs).
	Detail *httpinbound.DetailHandlers
	// Diff serves the diff tab of a task and the view its layout controls swap.
	Diff *httpinbound.DiffHandlers
	// Composer serves the message box of the detail page: the message, the mode switch and the file suggestions.
	Composer *httpinbound.ComposerHandlers
	// The dependency graph of the board (slice C): the edges, the node positions and the successor trigger.
	// Successors starts the tasks whose predecessors met their trigger. The composition root runs
	// background.RunGraphLoop with it in a goroutine it owns, and stops that loop through its context.
	AddDependency    *application.AddTaskDependency
	RemoveDependency *application.RemoveTaskDependency
	DetachFromGraph  *application.DetachTaskFromGraph
	GetGraph         *application.GetTaskGraph
	SetPosition      *application.SetTaskPosition
	Successors       *application.TriggerSuccessors
	// The resume of the tasks stopped by a usage limit (slice G). ResumeTask is the manual resume and
	// ResumeRateLimited is the sweep the resume loop runs (background.RunResumeLoop).
	ResumeTask        *application.ResumeTask
	ResumeRateLimited *application.ResumeRateLimitedTasks

	runtime outbound.AgentRuntime
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
	gitAdapter, err := git.NewWorktreeAdapter(deps.WorktreesDir)
	if err != nil {
		return nil, fmt.Errorf("tasks module: %w", err)
	}
	agentRuntime, err := newAgentRuntime(deps.Binaries)
	if err != nil {
		return nil, fmt.Errorf("tasks module: %w", err)
	}
	// Diffs are read through the cache; every other git call goes to the adapter underneath.
	worktrees := git.NewCachingWorktree(gitAdapter)
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
	graph := persistence.NewTaskGraphRepository(deps.DB, persistence.Clock(clock))
	attachments := persistence.NewTaskAttachmentRepository(deps.DB, persistence.Clock(clock))
	taskFiles := files.NewFsTaskFile()
	getAttachment := application.NewGetTaskAttachment(attachments, attachmentStorage)
	cleanup := application.NewAttachmentCleanup(attachments, attachmentStorage)
	eventSink := application.NewQueuedAgentEventSink(agentEvents, application.DefaultEventQueueCapacity)

	// The start use case is the one the UI and the graph trigger share, so that every start claims the task the same way.
	start := application.NewStartTask(lifecycle, worktrees, agentRuntime, deps.LaunchSettings, eventSink, attachments, attachmentStorage, newUUID, clock)
	successors := application.NewTriggerSuccessors(repository, graph, start, clock)
	send := application.NewSendMessage(lifecycle, agentRuntime)
	resumeTask := application.NewResumeTask(lifecycle, worktrees, agentRuntime, deps.LaunchSettings, eventSink)
	resumeRateLimited := application.NewResumeRateLimitedTasks(lifecycle, agentRuntime, resumeTask, send)
	// The create, edit and delete of the board are the same use cases the module exposes; each is built once.
	createTask := application.NewCreateTask(lifecycle, worktrees, start, attachments, attachmentStorage, newUUID, clock)
	updateTask := application.NewUpdateTask(lifecycle, attachments, attachmentStorage, newUUID)
	deleteTask := application.NewDeleteTask(lifecycle, attachmentStorage)
	addDependency := application.NewAddTaskDependency(lifecycle, graph, successors)
	listAttachments := application.NewListTaskAttachments(attachments)
	board := httpinbound.NewBoardHandlers(httpinbound.BoardUseCases{
		List:            application.NewListTasks(repository),
		ListDone:        application.NewListDoneTasks(repository),
		Get:             getTask,
		CreateLinked:    application.NewCreateLinkedTask(createTask, addDependency, application.NewGetTaskGraph(graph)),
		Update:          updateTask,
		Start:           start,
		Delete:          deleteTask,
		ListAttachments: listAttachments,
		ListBranches:    application.NewListWorkspaceBranches(lifecycle, worktrees),
		Workspaces:      deps.Workspaces,
	}, clock)
	abortTask := application.NewAbortTask(lifecycle, agentRuntime)
	finishTask := application.NewFinishTask(lifecycle, worktrees, agentRuntime, cleanup)
	resetTask := application.NewResetTask(lifecycle, worktrees, agentRuntime)
	discardTask := application.NewDiscardTask(lifecycle, worktrees, agentRuntime)
	detail := httpinbound.NewDetailHandlers(httpinbound.DetailUseCases{
		Get:             getTask,
		Events:          getEvents,
		ListAttachments: listAttachments,
		Start:           start,
		Abort:           abortTask,
		Resume:          resumeTask,
		Finish:          finishTask,
		Reset:           resetTask,
		Discard:         discardTask,
		Delete:          deleteTask,
		Panels:          deps.Panels,
	}, clock)
	diff := httpinbound.NewDiffHandlers(httpinbound.DiffUseCases{
		Get:  getTask,
		Diff: application.NewGetTaskDiff(lifecycle, worktrees),
	})
	setAgentMode := application.NewSetAgentMode(lifecycle, agentRuntime)
	answerDialog := application.NewAnswerDialog(lifecycle, agentRuntime)
	composer := httpinbound.NewComposerHandlers(httpinbound.ComposerUseCases{
		Send:    send,
		Mode:    setAgentMode,
		History: getEvents,
		Answer:  answerDialog,
	}, deps.FileSearch)

	return &Module{
		Get:               getTask,
		List:              application.NewListTasks(repository),
		ListDone:          application.NewListDoneTasks(repository),
		Create:            createTask,
		Update:            updateTask,
		Delete:            deleteTask,
		AnswerDialog:      answerDialog,
		GetSubagents:      application.NewGetTaskSubagents(lifecycle, agentRuntime),
		DeleteWorkspace:   application.NewDeleteWorkspaceTasks(lifecycle, worktrees, attachmentStorage),
		Start:             start,
		Send:              send,
		SetAgentMode:      setAgentMode,
		Abort:             abortTask,
		Finish:            finishTask,
		Reset:             resetTask,
		Discard:           discardTask,
		Reconcile:         application.NewReconcileTasks(lifecycle, worktrees),
		ListAttachments:   listAttachments,
		GetAttachment:     getAttachment,
		Cleanup:           cleanup,
		Attachments:       httpinbound.NewAttachmentHandler(getAttachment),
		GetFile:           application.NewGetTaskFile(lifecycle, taskFiles),
		SaveFile:          application.NewSaveTaskFile(lifecycle, taskFiles, gitAdapter),
		Worktrees:         worktrees,
		GetDiff:           application.NewGetTaskDiff(lifecycle, worktrees),
		Merge:             application.NewMergeTask(lifecycle, worktrees, successors, deps.Remote),
		SweepWorktrees:    application.NewSweepOrphanWorktrees(repository, worktrees),
		ListBranches:      application.NewListWorkspaceBranches(lifecycle, worktrees),
		Lifecycle:         lifecycle,
		AgentEvents:       agentEvents,
		GetEvents:         getEvents,
		EventSink:         eventSink,
		Subscriber:        bus,
		Streams:           httpinbound.NewTaskStreams(getTask, getEvents, bus),
		Board:             board,
		Detail:            detail,
		Diff:              diff,
		Composer:          composer,
		AddDependency:     addDependency,
		RemoveDependency:  application.NewRemoveTaskDependency(graph, successors),
		DetachFromGraph:   application.NewDetachTaskFromGraph(graph, successors),
		GetGraph:          application.NewGetTaskGraph(graph),
		SetPosition:       application.NewSetTaskPosition(graph),
		Successors:        successors,
		ResumeTask:        resumeTask,
		ResumeRateLimited: resumeRateLimited,
		runtime:           agentRuntime,
	}, nil
}

// newAgentRuntime builds the router over the four runtime adapters. An unknown runtime identifier is routed
// to the default (gentle-shell), as the Rust router does.
func newAgentRuntime(binaries RuntimeBinaries) (outbound.AgentRuntime, error) {
	gentleShellID, err := parseRuntimeID(runtimeGentleShell)
	if err != nil {
		return nil, err
	}
	piID, err := parseRuntimeID(runtimePi)
	if err != nil {
		return nil, err
	}
	claudeID, err := parseRuntimeID(runtimeClaude)
	if err != nil {
		return nil, err
	}
	cursorID, err := parseRuntimeID(runtimeCursor)
	if err != nil {
		return nil, err
	}
	runtimes := map[models.RuntimeID]outbound.AgentRuntime{
		gentleShellID: pi.NewRpcRuntimeAdapter(binaries.GentleShell),
		piID:          pi.NewRpcRuntimeAdapter(binaries.Pi),
		claudeID:      claude.NewStreamRuntimeAdapter(binaries.Claude),
		cursorID:      cursor.NewCursorAcpRuntimeAdapter(binaries.Cursor, binaries.CursorSessionsDir),
	}
	router, err := runtime.NewAgentRuntimeRouter(gentleShellID, runtimes)
	if err != nil {
		return nil, fmt.Errorf("agent runtime router: %w", err)
	}
	return router, nil
}

// parseRuntimeID reads the identifier of a runtime the module names itself.
func parseRuntimeID(raw string) (models.RuntimeID, error) {
	id, err := models.ParseRuntimeID(raw)
	if err != nil {
		return models.RuntimeID{}, fmt.Errorf("runtime %q: %w", raw, err)
	}
	return id, nil
}

// errMissingDependency is returned when a required dependency of the module is nil or empty.
var errMissingDependency = errors.New("tasks module: missing dependency")

// validate reports the first required dependency that is nil or empty.
func (d Deps) validate() error {
	required := []struct {
		name    string
		missing bool
	}{
		{"DB", d.DB == nil},
		{"Workspaces", d.Workspaces == nil},
		{"AttachmentsDir", d.AttachmentsDir == ""},
		{"WorktreesDir", d.WorktreesDir == ""},
		{"Binaries.GentleShell", d.Binaries.GentleShell == ""},
		{"Binaries.Pi", d.Binaries.Pi == ""},
		{"Binaries.Claude", d.Binaries.Claude == ""},
		{"Binaries.Cursor", d.Binaries.Cursor == ""},
		{"Binaries.CursorSessionsDir", d.Binaries.CursorSessionsDir == ""},
		{"LaunchSettings", d.LaunchSettings == nil},
	}
	for _, dependency := range required {
		if dependency.missing {
			return fmt.Errorf("%w: %s", errMissingDependency, dependency.name)
		}
	}
	return nil
}

// Routes mounts the live streams and the attachment content. It satisfies web.Module, so the composition root
// mounts it under the same access and token middlewares as the other modules.
func (m *Module) Routes(r chi.Router) {
	r.Get(httpinbound.TaskEventsRoute, m.Streams.TaskEvents)
	r.Get(httpinbound.BoardEventsRoute, m.Streams.BoardEvents)
	r.Get(httpinbound.AttachmentRoute, m.Attachments.Serve)
	m.Board.Mount(r)
	m.Detail.Mount(r)
	m.Diff.Mount(r)
	m.Composer.Mount(r)
}

// Sidebar returns the board part of the sidebar: the workspaces and the tasks that need the user.
func (m *Module) Sidebar(ctx context.Context) templ.Component {
	return m.Board.Sidebar(ctx)
}

// Tabs returns the dock routes the module owns: the board, at the home address and per workspace, the detail
// and the diff of a task. The composition root registers them before the not-migrated catch-alls.
func (m *Module) Tabs() []dock.Route {
	return append(append(m.Board.Tabs(), m.Detail.Tabs()...), m.Diff.Tabs()...)
}

// StopAgents stops every live agent. Call it on shutdown, after the background loops have stopped, so that no
// agent outlives the process. It is best effort.
func (m *Module) StopAgents(ctx context.Context) {
	m.runtime.StopAll(ctx)
}

// WaitWorktreeCleanup waits for the background removals of worktrees started so far. Call it on shutdown,
// after the worktree removals are done, so that no trashed folder is left behind.
func (m *Module) WaitWorktreeCleanup(ctx context.Context) {
	m.Worktrees.WaitForCleanup(ctx)
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
