package application_test

import (
	"context"
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
	detailsResult  *bool
	deleteResult   *bool
	activeFilter   *string
	donePage       [2]int64
	doneWorkspace  *string
	eventsTask     models.TaskID
	diffRanges     []models.DiffRange
	recordRangeErr error
	eventsAfter    *int64
	insertErr      error
	appendErr      error
}

func newFakeRepository(tasks ...models.Task) *fakeRepository {
	repo := &fakeRepository{tasks: map[models.TaskID]models.Task{}}
	for _, task := range tasks {
		repo.tasks[task.ID] = task
	}
	return repo
}

func (r *fakeRepository) Insert(_ context.Context, task models.Task) error {
	if r.insertErr != nil {
		return r.insertErr
	}
	r.tasks[task.ID] = task
	return nil
}

func (r *fakeRepository) Find(_ context.Context, id models.TaskID) (*models.Task, error) {
	task, ok := r.tasks[id]
	if !ok {
		return nil, nil
	}
	return &task, nil
}

func (r *fakeRepository) UpdateStatus(_ context.Context, id models.TaskID, status models.TaskStatus, errorText *string) error {
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

func (r *fakeRepository) UpdateDetailsIfBacklog(_ context.Context, id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string) (bool, error) {
	if r.detailsResult != nil {
		return *r.detailsResult, nil
	}
	task, ok := r.tasks[id]
	if !ok || task.Status != models.StatusBacklog {
		return false, nil
	}
	task.Title, task.Prompt, task.Priority, task.Limits, task.BaseBranch = title, prompt, priority, limits, baseBranch
	r.tasks[id] = task
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

// fakeRuntime is the agent runtime seen by the use cases that answer dialogs or list subagents.
type fakeRuntime struct {
	outbound.AgentRuntime
	running       bool
	capabilities  models.AgentCapabilities
	sent          []models.AgentCommand
	subagents     []models.SubagentTask
	subagentCalls int
}

func (r *fakeRuntime) IsRunning(_ context.Context, _ models.TaskID) bool { return r.running }

func (r *fakeRuntime) Capabilities(_ models.TaskID) models.AgentCapabilities {
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
