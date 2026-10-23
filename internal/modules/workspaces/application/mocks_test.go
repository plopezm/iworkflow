package application_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
)

// fixedNow is the instant every test uses as "now".
var fixedNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// newFixedID returns a fresh identifier; tests that need a known one build it themselves.
func newFixedID() (models.WorkspaceID, error) { return models.NewWorkspaceID() }

// fakeRepository is an in-memory WorkspaceRepository. Set the *Err fields to inject failures.
type fakeRepository struct {
	items     []models.Workspace
	saveErr   error
	listErr   error
	findErr   error
	updateErr error
	// updateFailFrom makes Update fail from its N-th call (1-based) on; zero means never.
	updateFailFrom int
	deleteErr      error
	saved          []models.Workspace
	updated        []models.Workspace
	deleted        []models.WorkspaceID
}

func newFakeRepository(items ...models.Workspace) *fakeRepository {
	return &fakeRepository{items: append([]models.Workspace(nil), items...)}
}

func (r *fakeRepository) Save(_ context.Context, workspace models.Workspace) error {
	r.saved = append(r.saved, workspace)
	if r.saveErr != nil {
		return r.saveErr
	}
	r.items = append(r.items, workspace)
	return nil
}

func (r *fakeRepository) List(_ context.Context) ([]models.Workspace, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return append([]models.Workspace(nil), r.items...), nil
}

func (r *fakeRepository) Update(_ context.Context, workspace models.Workspace) error {
	r.updated = append(r.updated, workspace)
	if r.updateErr != nil {
		return r.updateErr
	}
	if r.updateFailFrom > 0 && len(r.updated) >= r.updateFailFrom {
		return werrors.Repository("revert boom")
	}
	for index, item := range r.items {
		if item.ID == workspace.ID {
			r.items[index].Name = workspace.Name
			r.items[index].BaseBranch = workspace.BaseBranch
			return nil
		}
	}
	return werrors.NotFound(workspace.ID.String())
}

func (r *fakeRepository) FindByID(_ context.Context, id models.WorkspaceID) (*models.Workspace, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	for _, item := range r.items {
		if item.ID == id {
			found := item
			return &found, nil
		}
	}
	return nil, nil
}

func (r *fakeRepository) FindByPath(_ context.Context, path string) (*models.Workspace, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	for _, item := range r.items {
		if item.Path == path {
			found := item
			return &found, nil
		}
	}
	return nil, nil
}

func (r *fakeRepository) Delete(_ context.Context, id models.WorkspaceID) error {
	r.deleted = append(r.deleted, id)
	if r.deleteErr != nil {
		return r.deleteErr
	}
	for index, item := range r.items {
		if item.ID == id {
			r.items = append(r.items[:index], r.items[index+1:]...)
			return nil
		}
	}
	return werrors.NotFound(id.String())
}

// fakeGit is an in-memory GitRepository. repos maps a path to its state; a missing path yields
// PathNotFound, and a path present as a plain folder yields NotAGitRepository.
type fakeGit struct {
	repos        map[string]models.GitRepositoryInfo
	plainFolders map[string]bool
	branches     map[string]bool
	inspectErr   error
	branchErr    error
	initErr      error
	commitErr    error
	inits        []string
	initBranches []string
	commits      []string
}

func newFakeGit() *fakeGit {
	return &fakeGit{
		repos:        map[string]models.GitRepositoryInfo{},
		plainFolders: map[string]bool{},
		branches:     map[string]bool{},
	}
}

func (g *fakeGit) Inspect(_ context.Context, path string) (models.GitRepositoryInfo, error) {
	if g.inspectErr != nil {
		return models.GitRepositoryInfo{}, g.inspectErr
	}
	if info, ok := g.repos[path]; ok {
		return info, nil
	}
	if g.plainFolders[path] {
		return models.GitRepositoryInfo{}, werrors.NotAGitRepository(path)
	}
	return models.GitRepositoryInfo{}, werrors.PathNotFound(path)
}

func (g *fakeGit) BranchExists(_ context.Context, root string, branch string) (bool, error) {
	if g.branchErr != nil {
		return false, g.branchErr
	}
	return g.branches[root+"|"+branch], nil
}

func (g *fakeGit) InitRepository(_ context.Context, path string, initialBranch string) error {
	g.inits = append(g.inits, path)
	g.initBranches = append(g.initBranches, initialBranch)
	if g.initErr != nil {
		return g.initErr
	}
	delete(g.plainFolders, path)
	g.repos[path] = models.GitRepositoryInfo{Root: path, CurrentBranch: initialBranch, HasCommits: false}
	return nil
}

func (g *fakeGit) CreateInitialCommit(_ context.Context, root string) error {
	g.commits = append(g.commits, root)
	if g.commitErr != nil {
		return g.commitErr
	}
	info := g.repos[root]
	info.HasCommits = true
	g.repos[root] = info
	return nil
}

// fakeDirectoryBrowser is an in-memory DirectoryBrowser that records the calls it receives.
type fakeDirectoryBrowser struct {
	listed     []string
	listResult models.DirectoryListing
	listErr    error
	created    [][2]string
	createErr  error
}

func (b *fakeDirectoryBrowser) List(_ context.Context, path string) (models.DirectoryListing, error) {
	b.listed = append(b.listed, path)
	if b.listErr != nil {
		return models.DirectoryListing{}, b.listErr
	}
	if b.listResult.Path == "" {
		return models.DirectoryListing{Path: path, Parent: filepath.Dir(path)}, nil
	}
	return b.listResult, nil
}

func (b *fakeDirectoryBrowser) CreateDirectory(_ context.Context, parent string, name string) (string, error) {
	b.created = append(b.created, [2]string{parent, name})
	if b.createErr != nil {
		return "", b.createErr
	}
	return filepath.Join(parent, name), nil
}

// fakeAgentsFiles is an in-memory AgentsFiles port.
type fakeAgentsFiles struct {
	options    []models.AgentsFileOption
	listErr    error
	installErr error
	installs   []installCall
}

type installCall struct {
	id        string
	root      string
	overwrite bool
}

func (a *fakeAgentsFiles) ListOptions(_ context.Context) ([]models.AgentsFileOption, error) {
	if a.listErr != nil {
		return nil, a.listErr
	}
	return a.options, nil
}

func (a *fakeAgentsFiles) Install(_ context.Context, agentsFileID string, root string, overwrite bool) error {
	a.installs = append(a.installs, installCall{id: agentsFileID, root: root, overwrite: overwrite})
	return a.installErr
}

// fakeWorkspaceTasks records the workspaces whose tasks were deleted.
type fakeWorkspaceTasks struct {
	err     error
	deleted []models.WorkspaceID
}

func (t *fakeWorkspaceTasks) DeleteAll(_ context.Context, id models.WorkspaceID) error {
	t.deleted = append(t.deleted, id)
	return t.err
}

// workspaceAt builds a stored workspace whose path is the given root.
func workspaceAt(t testing.TB, name string, root string, branch string) models.Workspace {
	id, err := models.NewWorkspaceID()
	if err != nil {
		t.Fatalf("new workspace id: %v", err)
	}
	return models.Workspace{ID: id, Name: name, Path: root, BaseBranch: branch, CreatedAt: fixedNow}
}
