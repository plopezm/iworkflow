package application_test

import (
	"context"
	"testing"
	"time"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// The fakes below are the hand-written counterparts of the mockall mocks of the Rust tests. Each one
// keeps a small state, can be told to fail, and records its calls so tests can assert "never called".

// fakeConnections is an in-memory ConnectionRepository.
type fakeConnections struct {
	stored    *models.Connection
	getErr    error
	saveErr   error
	deleteErr error
	recordErr error
	saved     []models.Connection
	deleted   int
	syncs     []recordedSync
}

type recordedSync struct {
	at    time.Time
	error *string
}

var _ outbound.ConnectionRepository = (*fakeConnections)(nil)

func (f *fakeConnections) Get(_ context.Context) (*models.Connection, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.stored == nil {
		return nil, nil
	}
	copied := *f.stored
	return &copied, nil
}

func (f *fakeConnections) Save(_ context.Context, connection models.Connection) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, connection)
	f.stored = &connection
	return nil
}

func (f *fakeConnections) Delete(_ context.Context) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted++
	f.stored = nil
	return nil
}

func (f *fakeConnections) RecordSync(_ context.Context, at time.Time, syncError *string) error {
	f.syncs = append(f.syncs, recordedSync{at: at, error: syncError})
	return f.recordErr
}

// fakeIssues is an in-memory IssueRepository. ReplaceSynced only records its input; the adapter tests
// cover its storage semantics.
type fakeIssues struct {
	issues       []models.Issue
	findErr      error
	deleteAllErr error
	replaceErr   error
	replaceAdded int
	deleteAll    int
	replaced     [][]models.LaneBatch
	updated      []models.RemoteIssue
	updateErr    error
	setStates    []setStateCall
	setErr       error
}

type setStateCall struct {
	key    string
	state  models.IssueLocalState
	taskID *string
}

var _ outbound.IssueRepository = (*fakeIssues)(nil)

func (f *fakeIssues) List(_ context.Context) ([]models.Issue, error) {
	return append([]models.Issue(nil), f.issues...), nil
}

func (f *fakeIssues) Find(_ context.Context, key string) (*models.Issue, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	for index := range f.issues {
		if f.issues[index].Key() == key {
			found := f.issues[index]
			return &found, nil
		}
	}
	return nil, nil
}

func (f *fakeIssues) LaneMemberships(_ context.Context) (map[string][]string, error) {
	memberships := make(map[string][]string)
	for _, issue := range f.issues {
		memberships["mine"] = append(memberships["mine"], issue.Key())
	}
	return memberships, nil
}

func (f *fakeIssues) ReplaceSynced(_ context.Context, batches []models.LaneBatch, _ time.Time) (int, error) {
	f.replaced = append(f.replaced, batches)
	if f.replaceErr != nil {
		return 0, f.replaceErr
	}
	return f.replaceAdded, nil
}

func (f *fakeIssues) UpdateRemote(_ context.Context, issue models.RemoteIssue, _ time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = append(f.updated, issue)
	for index := range f.issues {
		if f.issues[index].Key() == issue.Key {
			f.issues[index].Remote = issue
		}
	}
	return nil
}

func (f *fakeIssues) SetLocalState(_ context.Context, key string, state models.IssueLocalState, taskID *string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.setStates = append(f.setStates, setStateCall{key: key, state: state, taskID: taskID})
	for index := range f.issues {
		if f.issues[index].Key() == key {
			f.issues[index].LocalState = state
			f.issues[index].TaskID = taskID
			return nil
		}
	}
	return jiraerrors.IssueNotFound(key)
}

func (f *fakeIssues) DeleteAll(_ context.Context) error {
	f.deleteAll++
	return f.deleteAllErr
}

// fakeLanes is an in-memory LaneRepository.
type fakeLanes struct {
	lanes     []models.Lane
	listErr   error
	findErr   error
	createErr error
	created   []models.LaneDefinition
	updated   []models.LaneDefinition
	deleted   []string
	collapsed map[string]bool
}

var _ outbound.LaneRepository = (*fakeLanes)(nil)

func (f *fakeLanes) List(_ context.Context) ([]models.Lane, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]models.Lane(nil), f.lanes...), nil
}

func (f *fakeLanes) Find(_ context.Context, id string) (*models.Lane, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	for index := range f.lanes {
		if f.lanes[index].ID == id {
			found := f.lanes[index]
			return &found, nil
		}
	}
	return nil, nil
}

func (f *fakeLanes) Create(_ context.Context, definition models.LaneDefinition) (models.Lane, error) {
	if f.createErr != nil {
		return models.Lane{}, f.createErr
	}
	f.created = append(f.created, definition)
	lane := models.Lane{ID: "new", Name: definition.Name(), JQL: definition.JQL()}
	f.lanes = append(f.lanes, lane)
	return lane, nil
}

func (f *fakeLanes) Update(_ context.Context, id string, definition models.LaneDefinition) (models.Lane, error) {
	f.updated = append(f.updated, definition)
	for index := range f.lanes {
		if f.lanes[index].ID == id {
			f.lanes[index].Name = definition.Name()
			f.lanes[index].JQL = definition.JQL()
			return f.lanes[index], nil
		}
	}
	return models.Lane{}, jiraerrors.LaneNotFound(id)
}

func (f *fakeLanes) Delete(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeLanes) SetCollapsed(_ context.Context, id string, collapsed bool) error {
	if f.collapsed == nil {
		f.collapsed = make(map[string]bool)
	}
	f.collapsed[id] = collapsed
	return nil
}

// fakeClient is a scripted JiraClient. Each method calls its hook when set, otherwise it succeeds
// with zero values. Calls are counted per method name.
type fakeClient struct {
	calls          map[string]int
	verifyFn       func() (string, error)
	searchFn       func(jql string) ([]models.RemoteIssue, error)
	getIssueFn     func(key string) (models.RemoteIssue, error)
	commentsFn     func(key string) ([]models.Comment, error)
	addCommentFn   func(key string, body string) error
	transitionsFn  func(key string) ([]models.Transition, error)
	transitionFn   func(key string, transitionID string) error
	searchedJQL    []string
	postedComments []string
}

var _ outbound.JiraClient = (*fakeClient)(nil)

func newFakeClient() *fakeClient {
	return &fakeClient{calls: make(map[string]int)}
}

func (f *fakeClient) count(method string) {
	f.calls[method]++
}

func (f *fakeClient) Verify(_ context.Context, _ models.Connection) (string, error) {
	f.count("verify")
	if f.verifyFn != nil {
		return f.verifyFn()
	}
	return "", nil
}

func (f *fakeClient) Search(_ context.Context, _ models.Connection, jql string) ([]models.RemoteIssue, error) {
	f.count("search")
	f.searchedJQL = append(f.searchedJQL, jql)
	if f.searchFn != nil {
		return f.searchFn(jql)
	}
	return nil, nil
}

func (f *fakeClient) GetIssue(_ context.Context, _ models.Connection, key string) (models.RemoteIssue, error) {
	f.count("get_issue")
	if f.getIssueFn != nil {
		return f.getIssueFn(key)
	}
	return models.RemoteIssue{Key: key}, nil
}

func (f *fakeClient) Comments(_ context.Context, _ models.Connection, key string) ([]models.Comment, error) {
	f.count("comments")
	if f.commentsFn != nil {
		return f.commentsFn(key)
	}
	return nil, nil
}

func (f *fakeClient) AddComment(_ context.Context, _ models.Connection, key string, body string) error {
	f.count("add_comment")
	f.postedComments = append(f.postedComments, key+":"+body)
	if f.addCommentFn != nil {
		return f.addCommentFn(key, body)
	}
	return nil
}

func (f *fakeClient) Transitions(_ context.Context, _ models.Connection, key string) ([]models.Transition, error) {
	f.count("transitions")
	if f.transitionsFn != nil {
		return f.transitionsFn(key)
	}
	return nil, nil
}

func (f *fakeClient) Transition(_ context.Context, _ models.Connection, key string, transitionID string) error {
	f.count("transition")
	if f.transitionFn != nil {
		return f.transitionFn(key, transitionID)
	}
	return nil
}

// fakeTasks records the task creations and answers with the configured outcome.
type fakeTasks struct {
	calls     []outbound.NewLinkedTask
	createErr error
	result    outbound.CreatedLinkedTask
}

var _ outbound.TaskCreation = (*fakeTasks)(nil)

func (f *fakeTasks) Create(_ context.Context, task outbound.NewLinkedTask) (outbound.CreatedLinkedTask, error) {
	f.calls = append(f.calls, task)
	if f.createErr != nil {
		return outbound.CreatedLinkedTask{}, f.createErr
	}
	return f.result, nil
}

// fakeWorkspaces is a scripted WorkspaceCatalog.
type fakeWorkspaces struct {
	workspaces []models.WorkspaceOption
	branches   []models.BranchOption
	branchErr  error
	branchOf   []string
}

var _ outbound.WorkspaceCatalog = (*fakeWorkspaces)(nil)

func (f *fakeWorkspaces) Workspaces(_ context.Context) ([]models.WorkspaceOption, error) {
	return f.workspaces, nil
}

func (f *fakeWorkspaces) Branches(_ context.Context, workspaceID string) ([]models.BranchOption, error) {
	f.branchOf = append(f.branchOf, workspaceID)
	if f.branchErr != nil {
		return nil, f.branchErr
	}
	return f.branches, nil
}

// fixedClock returns the same instant every time.
func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

// connectedRepo returns a connection repository that already holds a valid connection.
func connectedRepo(t testing.TB, url string, token string) *fakeConnections {
	t.Helper()
	connection, err := models.NewConnection(url, token, false)
	if err != nil {
		t.Fatalf("valid connection: %v", err)
	}
	return &fakeConnections{stored: &connection}
}

// sampleRemote is the Jira data of an issue, as the tests of the module use it.
func sampleRemote(key string) models.RemoteIssue {
	priority := "High"
	assignee := "Ana"
	reporter := "Luis"
	return models.RemoteIssue{
		Key:           key,
		Summary:       "Resumen de " + key,
		Description:   "Descripción larga",
		Status:        "To Do",
		IssueType:     "Bug",
		Priority:      &priority,
		Assignee:      &assignee,
		Reporter:      &reporter,
		Labels:        []string{"backend"},
		URL:           "https://jira.local/browse/" + key,
		JiraUpdatedAt: time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC),
	}
}

// sampleIssue is an imported, not yet converted issue.
func sampleIssue(key string) models.Issue {
	return models.Issue{Remote: sampleRemote(key), LocalState: models.IssueLocalStateNew, SyncedAt: syncAt}
}
