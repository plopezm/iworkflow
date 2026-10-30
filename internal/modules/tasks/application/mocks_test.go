package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// fixedNow is the instant every test uses as "now".
var fixedNow = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func clock() time.Time { return fixedNow }

func newUUID() (uuid.UUID, error) { return uuid.NewRandom() }

// newTask builds a task in the given status with a fresh id.
func newTask(status models.TaskStatus, dialogIDs ...string) models.Task {
	id, err := models.NewTaskID()
	if err != nil {
		panic(err)
	}
	dialogs := make([]models.PendingDialog, 0, len(dialogIDs))
	for _, dialogID := range dialogIDs {
		dialogs = append(dialogs, models.NewPendingDialog(dialogID, nil))
	}
	return models.Task{
		ID:             id,
		WorkspaceID:    "ws-1",
		Title:          "Arregla el login",
		Prompt:         "Arregla el login",
		Branch:         "pi/arregla-el-login-abc",
		BaseBranch:     "main",
		WorktreePath:   "/worktrees/demo/abc-task",
		Status:         status,
		Priority:       models.DefaultTaskPriority,
		PendingDialogs: dialogs,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
	}
}

// fakeRepository is an in-memory TaskRepository. Methods that a test does not use panic through the
// embedded nil interface, so an unexpected call fails loudly.
type fakeRepository struct {
	outbound.TaskRepository
	// finds counts the lookups; it is atomic because the trigger tests look tasks up concurrently.
	finds atomic.Int64

	tasks         map[models.TaskID]models.Task
	statuses      []models.TaskStatus
	errorTexts    []*string
	events        []models.TaskEventKind
	dialogAdds    []models.PendingDialog
	dialogRemoves []string
	dialogClears  int
	subagents     []models.SubagentActivity
	usageLimits   []models.UsageLimitState
	// detailsResult, when set, is returned by UpdateDetailsIfBacklog instead of the real check.
	detailsResult *bool
	// relatedByTask holds the related workspace ids written by UpdateDetailsAndRelatedIfBacklog.
	relatedByTask map[models.TaskID][]string
	// updateDetailsCalls counts UpdateDetailsAndRelatedIfBacklog calls; updateDetailsRelatedErr makes them fail
	// without changing anything, as a rolled-back transaction does.
	updateDetailsCalls      int
	insertCalls             int
	updateDetailsRelatedErr error
	deleteResult            *bool
	activeFilter            *string
	donePage                [2]int64
	doneWorkspace           *string
	eventsTask              models.TaskID
	diffRanges              []models.DiffRange
	recordRangeErr          error
	eventsAfter             *int64
	insertErr               error
	appendErr               error
	// claimResult and revertResult, when set, are returned instead of the real conditional check.
	claimResult  *bool
	claimErr     error
	revertResult *bool
	revertErr    error
	claims       int
	reverts      int
	// resetResult and resetErr, when set, are returned by ResetToBacklog; resets counts the calls.
	resetResult *bool
	resetErr    error
	resets      int
	// updateStatusErrs fails the status updates to the listed statuses, before anything is recorded.
	updateStatusErrs map[models.TaskStatus]error
}

func newFakeRepository(tasks ...models.Task) *fakeRepository {
	repo := &fakeRepository{tasks: map[models.TaskID]models.Task{}, relatedByTask: map[models.TaskID][]string{}}
	for _, task := range tasks {
		repo.tasks[task.ID] = task
	}
	return repo
}

func (r *fakeRepository) Insert(_ context.Context, task models.Task) error {
	r.insertCalls++
	if r.insertErr != nil {
		return r.insertErr
	}
	r.tasks[task.ID] = task
	return nil
}

func (r *fakeRepository) Find(_ context.Context, id models.TaskID) (*models.Task, error) {
	r.finds.Add(1)
	task, ok := r.tasks[id]
	if !ok {
		return nil, nil
	}
	return &task, nil
}

func (r *fakeRepository) UpdateStatus(_ context.Context, id models.TaskID, status models.TaskStatus, errorText *string) error {
	if err := r.updateStatusErrs[status]; err != nil {
		return err
	}
	task, ok := r.tasks[id]
	if !ok {
		return werrors.NotFound(id.String())
	}
	task.Status = status
	task.Error = errorText
	r.tasks[id] = task
	r.statuses = append(r.statuses, status)
	r.errorTexts = append(r.errorTexts, errorText)
	return nil
}

// ClaimForStart moves a backlog task to starting and fixes its plan, like the conditional update does.
func (r *fakeRepository) ClaimForStart(_ context.Context, id models.TaskID, branch string, worktreePath string, baseBranch string) (bool, error) {
	r.claims++
	if r.claimErr != nil {
		return false, r.claimErr
	}
	if r.claimResult != nil {
		return *r.claimResult, nil
	}
	task, ok := r.tasks[id]
	if !ok || task.Status != models.StatusBacklog {
		return false, nil
	}
	task.Status, task.Branch, task.WorktreePath, task.BaseBranch = models.StatusStarting, branch, worktreePath, baseBranch
	r.tasks[id] = task
	return true, nil
}

// RevertStartClaim moves a starting task back to backlog.
func (r *fakeRepository) RevertStartClaim(_ context.Context, id models.TaskID) (bool, error) {
	r.reverts++
	if r.revertErr != nil {
		return false, r.revertErr
	}
	if r.revertResult != nil {
		return *r.revertResult, nil
	}
	task, ok := r.tasks[id]
	if !ok || task.Status != models.StatusStarting {
		return false, nil
	}
	task.Status = models.StatusBacklog
	r.tasks[id] = task
	return true, nil
}

func (r *fakeRepository) UpdateDetailsIfBacklog(_ context.Context, id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string, agent models.TaskAgentOptions) (bool, error) {
	if r.detailsResult != nil {
		return *r.detailsResult, nil
	}
	task, ok := r.tasks[id]
	if !ok || task.Status != models.StatusBacklog {
		return false, nil
	}
	task.Title, task.Prompt, task.Priority, task.Limits, task.BaseBranch, task.Agent = title, prompt, priority, limits, baseBranch, agent
	r.tasks[id] = task
	return true, nil
}

func (r *fakeRepository) UpdateDetailsAndRelatedIfBacklog(_ context.Context, id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string, agent models.TaskAgentOptions, workspaceIDs []string) (bool, error) {
	r.updateDetailsCalls++
	if r.updateDetailsRelatedErr != nil {
		return false, r.updateDetailsRelatedErr
	}
	task, ok := r.tasks[id]
	if !ok || task.Status != models.StatusBacklog {
		return false, nil
	}
	task.Title, task.Prompt, task.Priority, task.Limits, task.BaseBranch, task.Agent = title, prompt, priority, limits, baseBranch, agent
	r.tasks[id] = task
	r.relatedByTask[id] = append([]string(nil), workspaceIDs...)
	return true, nil
}

func (r *fakeRepository) DeleteIfBacklog(_ context.Context, id models.TaskID) (bool, error) {
	if r.deleteResult != nil {
		return *r.deleteResult, nil
	}
	task, ok := r.tasks[id]
	if !ok || task.Status != models.StatusBacklog {
		return false, nil
	}
	delete(r.tasks, id)
	return true, nil
}

func (r *fakeRepository) AddPendingDialog(_ context.Context, _ models.TaskID, dialog models.PendingDialog) error {
	r.dialogAdds = append(r.dialogAdds, dialog)
	return nil
}

func (r *fakeRepository) RemovePendingDialog(_ context.Context, _ models.TaskID, dialogID string) error {
	r.dialogRemoves = append(r.dialogRemoves, dialogID)
	return nil
}

func (r *fakeRepository) ClearPendingDialogs(_ context.Context, _ models.TaskID) error {
	r.dialogClears++
	return nil
}

func (r *fakeRepository) SetSubagentActivity(_ context.Context, _ models.TaskID, activity models.SubagentActivity) error {
	r.subagents = append(r.subagents, activity)
	return nil
}

func (r *fakeRepository) SetUsageLimit(_ context.Context, _ models.TaskID, state models.UsageLimitState) error {
	r.usageLimits = append(r.usageLimits, state)
	return nil
}

func (r *fakeRepository) AppendEvent(_ context.Context, id models.TaskID, kind models.TaskEventKind) (models.TaskEvent, error) {
	if r.appendErr != nil {
		return models.TaskEvent{}, r.appendErr
	}
	r.events = append(r.events, kind)
	seq := int64(len(r.events))
	return models.TaskEvent{TaskID: id, Seq: &seq, Kind: kind, CreatedAt: fixedNow}, nil
}

func (r *fakeRepository) ListActive(_ context.Context, workspaceID *string) ([]models.Task, error) {
	r.activeFilter = workspaceID
	var tasks []models.Task
	for _, task := range r.tasks {
		if !task.Status.IsClosed() {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func (r *fakeRepository) Events(_ context.Context, id models.TaskID, afterSeq *int64) ([]models.TaskEvent, error) {
	r.eventsTask = id
	r.eventsAfter = afterSeq
	return nil, nil
}

func (r *fakeRepository) ListDonePage(_ context.Context, workspaceID *string, limit int64, offset int64) ([]models.Task, error) {
	r.donePage = [2]int64{limit, offset}
	r.doneWorkspace = workspaceID
	return nil, nil
}

func (r *fakeRepository) DeleteInactiveByWorkspace(_ context.Context, workspaceID string) ([]models.TaskID, bool, error) {
	var ids []models.TaskID
	for id, task := range r.tasks {
		if task.WorkspaceID != workspaceID {
			continue
		}
		if !task.Status.IsInactive() {
			return nil, false, nil
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		delete(r.tasks, id)
	}
	return ids, true, nil
}

// ResetToBacklog answers with resetResult or resetErr when set; otherwise the reset applies.
func (r *fakeRepository) ResetToBacklog(_ context.Context, _ models.TaskID) (bool, error) {
	r.resets++
	if r.resetErr != nil {
		return false, r.resetErr
	}
	if r.resetResult != nil {
		return *r.resetResult, nil
	}
	return true, nil
}

// fakePublisher records the published events.
type fakePublisher struct {
	published []models.TaskEvent
}

func (p *fakePublisher) Publish(event models.TaskEvent) {
	p.published = append(p.published, event)
}

// fakeWorkspaces knows one workspace, "ws-1", whose base branch is main.
type fakeWorkspaces struct{}

func (fakeWorkspaces) Find(_ context.Context, workspaceID string) (*models.WorkspaceRef, error) {
	if workspaceID != "ws-1" {
		return nil, nil
	}
	return &models.WorkspaceRef{ID: "ws-1", Name: "demo", Path: "/repos/demo", BaseBranch: "main"}, nil
}

func (fakeWorkspaces) List(_ context.Context) ([]models.WorkspaceRef, error) {
	return nil, nil
}

// fakeStorage records saved and deleted files. The failure fields inject errors.
type fakeStorage struct {
	outbound.AttachmentStorage
	saved       []string
	deleted     []string
	deletedDirs []models.TaskID
	failSave    bool
	// failSaveFrom makes Save fail from its N-th call (1-based) on; zero means never.
	failSaveFrom    int
	saveCalls       int
	failDelete      bool
	failDeleteDir   bool
	readBytesResult []byte
	readBytesErr    error
	readErr         error
	readPaths       []string
}

func (s *fakeStorage) Save(_ context.Context, taskID models.TaskID, attachmentID string, _ models.ImageAttachment) (string, error) {
	s.saveCalls++
	if s.failSaveFrom > 0 && s.saveCalls >= s.failSaveFrom {
		return "", werrors.Storage("sin espacio")
	}
	if s.failSave {
		return "", werrors.Storage("disco lleno")
	}
	path := taskID.String() + "/" + attachmentID + ".png"
	s.saved = append(s.saved, path)
	return path, nil
}

func (s *fakeStorage) Delete(_ context.Context, path string) error {
	s.deleted = append(s.deleted, path)
	if s.failDelete {
		return werrors.Storage("no se pudo borrar")
	}
	return nil
}

// Read answers with the base64 content the adapter would return: "data-<file name>" for the test paths.
func (s *fakeStorage) Read(_ context.Context, path string) (string, error) {
	if s.readErr != nil {
		return "", s.readErr
	}
	return "data-" + strings.TrimPrefix(path, "t/"), nil
}

// ReadBytes records the path it was asked for and answers with the configured content.
func (s *fakeStorage) ReadBytes(_ context.Context, path string) ([]byte, error) {
	s.readPaths = append(s.readPaths, path)
	return s.readBytesResult, s.readBytesErr
}

func (s *fakeStorage) DeleteTaskDir(_ context.Context, taskID models.TaskID) error {
	s.deletedDirs = append(s.deletedDirs, taskID)
	if s.failDeleteDir {
		return werrors.Storage("no se pudo borrar la carpeta")
	}
	return nil
}

// fakeAttachments records the metadata written for a task.
type fakeAttachments struct {
	outbound.TaskAttachmentRepository
	inserted         [][]models.StoredAttachment
	listed           []models.StoredAttachment
	removed          []models.StoredAttachment
	failInsert       bool
	deletedByTask    int
	failDeleteByTask error
	// listErr fails ListByTask, as the start tests need.
	listErr error
}

func (a *fakeAttachments) InsertMany(_ context.Context, _ models.TaskID, attachments []models.StoredAttachment) error {
	if a.failInsert {
		return werrors.Repository("insert failed")
	}
	a.inserted = append(a.inserted, attachments)
	return nil
}

// DeleteByTask records the call; failDeleteByTask injects a repository failure.
func (a *fakeAttachments) DeleteByTask(_ context.Context, _ models.TaskID) error {
	a.deletedByTask++
	return a.failDeleteByTask
}

func (a *fakeAttachments) ListByTask(_ context.Context, _ models.TaskID) ([]models.StoredAttachment, error) {
	if a.listErr != nil {
		return nil, a.listErr
	}
	return a.listed, nil
}

func (a *fakeAttachments) Remove(_ context.Context, _ models.TaskID, ids []string) ([]models.StoredAttachment, error) {
	var removed []models.StoredAttachment
	for _, attachment := range a.listed {
		for _, id := range ids {
			if attachment.ID == id {
				removed = append(removed, attachment)
			}
		}
	}
	a.removed = removed
	return removed, nil
}

// fakeLayout computes worktree paths without touching git.
type fakeLayout struct{}

func (fakeLayout) WorktreePath(workspaceName string, dirName string) string {
	return "/worktrees/" + workspaceName + "/" + dirName
}

// fakeStarter records the starts it receives and answers with a fixed error (nil means success).
type fakeStarter struct {
	started []models.TaskID
	err     error
}

func (s *fakeStarter) Start(_ context.Context, id models.TaskID) (models.Task, error) {
	s.started = append(s.started, id)
	if s.err != nil {
		return models.Task{}, s.err
	}
	return newTask(models.StatusStarting), nil
}

// edgeOf is a merged-trigger edge of workspace "ws-1", the workspace of newTask.
func edgeOf(predecessor models.TaskID, successor models.TaskID) models.TaskDependency {
	return models.TaskDependency{Predecessor: predecessor, Successor: successor, WorkspaceID: "ws-1", Trigger: models.DependencyTriggerMerged}
}

// fakeGraph is an in-memory TaskGraphRepository. Like the adapter, InsertDependency validates the edge
// against the edges of its workspace and stores nothing when it is rejected. Positions are not scoped by
// workspace here: the use cases pass the workspace, and the tests check that argument. It is safe for
// concurrent use because the trigger tests evaluate the graph from several goroutines.
type fakeGraph struct {
	mu sync.Mutex

	edges     []models.TaskDependency
	positions []models.TaskPosition
	// The errors are returned instead of the real work; nothing is stored then.
	insertErr error
	removeErr error
	listErr   error
	setErr    error
	detachErr error
	// inserts and sets count the writes that reached the graph, including the rejected ones.
	inserts int
	sets    int
	// dependencyQueries and positionQueries record the workspace argument of every read.
	dependencyQueries []*string
	positionQueries   []string
}

var _ outbound.TaskGraphRepository = (*fakeGraph)(nil)

func (g *fakeGraph) InsertDependency(_ context.Context, dependency models.TaskDependency) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.inserts++
	if g.insertErr != nil {
		return g.insertErr
	}
	if err := models.ValidateNewDependency(edgesOf(g.edges, dependency.WorkspaceID), dependency); err != nil {
		return err
	}
	g.edges = append(g.edges, dependency)
	return nil
}

func (g *fakeGraph) RemoveDependency(_ context.Context, predecessor models.TaskID, successor models.TaskID) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.removeErr != nil {
		return false, g.removeErr
	}
	for index, edge := range g.edges {
		if edge.Predecessor == predecessor && edge.Successor == successor {
			g.edges = append(g.edges[:index:index], g.edges[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func (g *fakeGraph) ListDependencies(_ context.Context, workspaceID *string) ([]models.TaskDependency, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.dependencyQueries = append(g.dependencyQueries, workspaceID)
	if g.listErr != nil {
		return nil, g.listErr
	}
	if workspaceID == nil {
		return slices.Clone(g.edges), nil
	}
	return edgesOf(g.edges, *workspaceID), nil
}

func (g *fakeGraph) SetPosition(_ context.Context, taskID models.TaskID, position models.NodePosition) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sets++
	if g.setErr != nil {
		return g.setErr
	}
	for index, stored := range g.positions {
		if stored.TaskID == taskID {
			g.positions[index].Position = position
			return nil
		}
	}
	g.positions = append(g.positions, models.TaskPosition{TaskID: taskID, Position: position})
	return nil
}

func (g *fakeGraph) DetachTask(_ context.Context, taskID models.TaskID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.detachErr != nil {
		return g.detachErr
	}
	var edges []models.TaskDependency
	for _, edge := range g.edges {
		if edge.Predecessor != taskID && edge.Successor != taskID {
			edges = append(edges, edge)
		}
	}
	g.edges = edges
	var positions []models.TaskPosition
	for _, stored := range g.positions {
		if stored.TaskID != taskID {
			positions = append(positions, stored)
		}
	}
	g.positions = positions
	return nil
}

func (g *fakeGraph) ListPositions(_ context.Context, workspaceID string) ([]models.TaskPosition, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.positionQueries = append(g.positionQueries, workspaceID)
	if g.listErr != nil {
		return nil, g.listErr
	}
	return slices.Clone(g.positions), nil
}

// edgesOf returns the edges of one workspace, in stored order.
func edgesOf(edges []models.TaskDependency, workspaceID string) []models.TaskDependency {
	var result []models.TaskDependency
	for _, edge := range edges {
		if edge.WorkspaceID == workspaceID {
			result = append(result, edge)
		}
	}
	return result
}

// graphTrigger counts the graph evaluations and answers with a fixed result.
type graphTrigger struct {
	calls  int
	result int
	err    error
}

func (f *graphTrigger) Trigger(_ context.Context) (int, error) {
	f.calls++
	return f.result, f.err
}

// fakeRuntime is the agent runtime seen by the use cases that answer dialogs or list subagents.
type fakeRuntime struct {
	outbound.AgentRuntime
	running       bool
	capabilities  models.AgentCapabilities
	sent          []models.AgentCommand
	subagents     []models.SubagentTask
	subagentCalls int
	// runningCalls and capabilityCalls count the questions asked, so tests can prove a check was skipped.
	runningCalls    int
	capabilityCalls int
	// stopped records the Stop calls; stopErr fails them.
	stopped []models.TaskID
	stopErr error
}

func (r *fakeRuntime) IsRunning(_ context.Context, _ models.TaskID) bool {
	r.runningCalls++
	return r.running
}

func (r *fakeRuntime) Capabilities(_ models.TaskID) models.AgentCapabilities {
	r.capabilityCalls++
	return r.capabilities
}

func (r *fakeRuntime) Send(_ context.Context, _ models.TaskID, command models.AgentCommand) error {
	r.sent = append(r.sent, command)
	return nil
}

func (r *fakeRuntime) Subagents(_ context.Context, _ models.TaskID) []models.SubagentTask {
	r.subagentCalls++
	return r.subagents
}

// Stop records the call and answers with stopErr.
func (r *fakeRuntime) Stop(_ context.Context, id models.TaskID) error {
	r.stopped = append(r.stopped, id)
	return r.stopErr
}

// newLifecycle wires the real lifecycle over the fakes and returns the publisher to inspect.
func newLifecycle(repo outbound.TaskRepository) (*application.TaskLifecycle, *fakePublisher) {
	publisher := &fakePublisher{}
	return application.NewTaskLifecycle(repo, publisher, fakeWorkspaces{}, clock), publisher
}

// newIDString returns a fresh UUID string for attachment identifiers in tests.
func newIDString(t *testing.T) string {
	t.Helper()
	value, err := uuid.NewRandom()
	require.NoError(t, err)
	return value.String()
}

// newTaskIDValue returns a fresh task identifier that no fake repository holds.
func newTaskIDValue(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return id
}

// fakeRemover records the worktree removals it is asked for and answers with failure when set.
type fakeRemover struct {
	removed []string
	err     error
}

func (r *fakeRemover) Remove(_ context.Context, repo string, path string, branch string) error {
	r.removed = append(r.removed, repo+"|"+path+"|"+branch)
	return r.err
}

// ListByStatus returns the stored tasks whose status is one of the given ones.
func (r *fakeRepository) ListByStatus(_ context.Context, statuses []models.TaskStatus) ([]models.Task, error) {
	var tasks []models.Task
	for _, task := range r.tasks {
		for _, status := range statuses {
			if task.Status == status {
				tasks = append(tasks, task)
				break
			}
		}
	}
	return tasks, nil
}

// RecordDiffRange stores the range for the assertions of the merge tests.
func (r *fakeRepository) RecordDiffRange(_ context.Context, _ models.TaskID, diffRange models.DiffRange) error {
	if r.recordRangeErr != nil {
		return r.recordRangeErr
	}
	r.diffRanges = append(r.diffRanges, diffRange)
	return nil
}

// List returns the stored tasks of one workspace, or of all of them when the filter is nil.
func (r *fakeRepository) List(_ context.Context, workspaceID *string) ([]models.Task, error) {
	var tasks []models.Task
	for _, task := range r.tasks {
		if workspaceID == nil || task.WorkspaceID == *workspaceID {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

// fakeRelatedProjects keeps the related projects of each task in memory. The failure fields inject errors.
type fakeRelatedProjects struct {
	stored       map[string][]string
	replaceCalls int
	failReplace  bool
	failList     bool
}

func newFakeRelatedProjects() *fakeRelatedProjects {
	return &fakeRelatedProjects{stored: map[string][]string{}}
}

func (f *fakeRelatedProjects) Replace(_ context.Context, id models.TaskID, workspaceIDs []string) error {
	f.replaceCalls++
	if f.failReplace {
		return errors.New("replace related projects failed")
	}
	f.stored[id.String()] = append([]string(nil), workspaceIDs...)
	return nil
}

func (f *fakeRelatedProjects) ListByTask(_ context.Context, id models.TaskID) ([]string, error) {
	if f.failList {
		return nil, errors.New("list related projects failed")
	}
	return append([]string(nil), f.stored[id.String()]...), nil
}

// knownWorkspaces is a workspace lookup over a fixed set of workspaces, keyed by id.
type knownWorkspaces map[string]models.WorkspaceRef

func (k knownWorkspaces) Find(_ context.Context, workspaceID string) (*models.WorkspaceRef, error) {
	ref, ok := k[workspaceID]
	if !ok {
		return nil, nil
	}
	return &ref, nil
}

func (k knownWorkspaces) List(_ context.Context) ([]models.WorkspaceRef, error) {
	return nil, nil
}
