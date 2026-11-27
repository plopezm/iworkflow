package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/workspaces/application"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

const (
	editedID      = "6f1c2c1e-0000-4000-8000-000000000001"
	storedRoot    = "/repo/demo"
	movedRoot     = "/moved/demo"
	anotherRepo   = "/other/repo"
	blockingTasks = true
)

// pathRepository is the workspace repository of the edit: one stored workspace and the one registered at each path.
type pathRepository struct {
	outbound.WorkspaceRepository
	stored  models.Workspace
	byPath  map[string]models.Workspace
	updates int
}

func (r *pathRepository) FindByID(context.Context, models.WorkspaceID) (*models.Workspace, error) {
	stored := r.stored
	return &stored, nil
}

func (r *pathRepository) FindByPath(_ context.Context, path string) (*models.Workspace, error) {
	if found, ok := r.byPath[path]; ok {
		return &found, nil
	}
	return nil, nil
}

func (r *pathRepository) Update(_ context.Context, workspace models.Workspace) error {
	r.updates++
	r.stored = workspace
	return nil
}

// pathGit answers the repositories the edit inspects: a registered root per folder, plain folders, and the roots.
type pathGit struct {
	outbound.GitRepository
	repos     map[string]models.GitRepositoryInfo
	plain     map[string]bool
	roots     map[string][]string
	linked    bool
	repairErr error
}

func (g *pathGit) Inspect(_ context.Context, path string) (models.GitRepositoryInfo, error) {
	if g.plain[path] {
		return models.GitRepositoryInfo{}, werrors.NotAGitRepository(path)
	}
	if info, ok := g.repos[path]; ok {
		return info, nil
	}
	return models.GitRepositoryInfo{}, werrors.PathNotFound(path)
}

func (g *pathGit) RootCommits(_ context.Context, root string) ([]string, error) {
	if commits, ok := g.roots[root]; ok {
		return commits, nil
	}
	return nil, werrors.PathNotFound(root)
}

func (g *pathGit) HasLinkedWorktrees(context.Context, string) (bool, error) { return g.linked, nil }

func (g *pathGit) RepairWorktrees(context.Context, string) error { return g.repairErr }

// pathTasks reports whether the workspace has unfinished tasks.
type pathTasks struct {
	outbound.WorkspaceTasks
	blocking bool
}

func (t pathTasks) HasBlocking(context.Context, models.WorkspaceID) (bool, error) {
	return t.blocking, nil
}

// editFixture is the workspace of the edit, stored at storedRoot, whose project is "first".
type editFixture struct {
	repository *pathRepository
	git        *pathGit
	tasks      pathTasks
}

func newEditFixture(t *testing.T) *editFixture {
	t.Helper()
	id, err := models.ParseWorkspaceID(editedID)
	require.NoError(t, err)
	workspace := models.Workspace{ID: id, Name: "Demo", Path: storedRoot, BaseBranch: "main"}
	return &editFixture{
		repository: &pathRepository{stored: workspace, byPath: map[string]models.Workspace{}},
		git: &pathGit{
			repos: map[string]models.GitRepositoryInfo{
				storedRoot: {Root: storedRoot, CurrentBranch: "main", HasCommits: true},
				movedRoot:  {Root: movedRoot, CurrentBranch: "main", HasCommits: true},
			},
			plain: map[string]bool{},
			roots: map[string][]string{storedRoot: {"first"}, movedRoot: {"first"}},
		},
	}
}

// postEdit posts the edit form to the update route and returns the answer.
func postEdit(t *testing.T, fixture *editFixture, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	use := application.NewUpdateWorkspace(fixture.repository, fixture.git, nil, nil, fixture.tasks)
	handlers := NewHandlers(UseCases{Update: use})
	router := chi.NewRouter()
	handlers.Mount(router)
	request := httptest.NewRequest(http.MethodPost, "/workspaces/"+editedID+"/update", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)
	return rec
}

func movedForm(path string) url.Values {
	return url.Values{"name": {"Demo"}, "path": {path}}
}

func TestUpdateRejectionsShowTheExactReasonAndSaveNothing(t *testing.T) {
	tests := []struct {
		name    string
		values  url.Values
		mutate  func(f *editFixture)
		want    string
		confirm bool
	}{
		{
			name:   "unfinished tasks",
			values: movedForm(movedRoot),
			mutate: func(f *editFixture) { f.tasks.blocking = blockingTasks },
			want:   "The path cannot change while the workspace has unfinished tasks.",
		},
		{
			name:   "relative path",
			values: movedForm("relative/demo"),
			want:   "The path must be absolute: relative/demo",
		},
		{
			name:   "folder missing",
			values: movedForm("/nowhere/demo"),
			want:   "The folder does not exist: /nowhere/demo",
		},
		{
			name:   "plain folder",
			values: movedForm("/plain/demo"),
			mutate: func(f *editFixture) { f.git.plain["/plain/demo"] = true },
			want:   "The folder is not a git repository: /plain/demo",
		},
		{
			name:   "subfolder of a repository",
			values: movedForm("/moved/demo/sub"),
			mutate: func(f *editFixture) {
				f.git.repos["/moved/demo/sub"] = models.GitRepositoryInfo{Root: movedRoot, CurrentBranch: "main", HasCommits: true}
			},
			want: "The new path is not the root of a git repository: /moved/demo/sub",
		},
		{
			name:   "another project",
			values: movedForm(anotherRepo),
			mutate: func(f *editFixture) {
				f.git.repos[anotherRepo] = models.GitRepositoryInfo{Root: anotherRepo, CurrentBranch: "main", HasCommits: true}
				f.git.roots[anotherRepo] = []string{"other"}
			},
			want: "The new path is not the same project as the workspace: /other/repo",
		},
		{
			name:   "path used by another workspace",
			values: movedForm(movedRoot),
			mutate: func(f *editFixture) {
				f.repository.byPath[movedRoot] = models.Workspace{Name: "Other", Path: movedRoot}
			},
			want: "The path is already registered by another workspace: /moved/demo",
		},
		{
			name:    "moved folder without the same project confirmation",
			values:  movedForm(movedRoot),
			mutate:  func(f *editFixture) { delete(f.git.roots, storedRoot) },
			want:    "The previous folder no longer exists, so the new folder cannot be compared; confirm it is the same project",
			confirm: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newEditFixture(t)
			if tc.mutate != nil {
				tc.mutate(fixture)
			}

			rec := postEdit(t, fixture, tc.values)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
			assert.NotContains(t, rec.Body.String(), "data-dock-redirect", "a rejection keeps the edit tab open")
			assert.Equal(t, 0, fixture.repository.updates, "a rejected edit saves nothing")
			assert.Equal(t, storedRoot, fixture.repository.stored.Path, "the stored path is unchanged")
			assert.Equal(t, tc.confirm, strings.Contains(rec.Body.String(), `name="confirm_same_project"`),
				"the confirmation checkbox is shown only for an unconfirmed move")
		})
	}
}

func TestUpdateConfirmedMoveIsSavedAndReturnsToTheList(t *testing.T) {
	fixture := newEditFixture(t)
	delete(fixture.git.roots, storedRoot)
	values := movedForm(movedRoot)
	values.Set("confirm_same_project", "true")

	rec := postEdit(t, fixture, values)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `data-dock-redirect="/workspaces"`)
	assert.Equal(t, 1, fixture.repository.updates)
	assert.Equal(t, movedRoot, fixture.repository.stored.Path)
}

func TestUpdateRepairFailureIsANoticeAndTheMoveStaysSaved(t *testing.T) {
	fixture := newEditFixture(t)
	fixture.git.linked = true
	fixture.git.repairErr = errors.New("worktree broke")

	rec := postEdit(t, fixture, movedForm(movedRoot))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "The path was saved, but git could not repair the worktrees: worktree broke")
	assert.NotContains(t, rec.Body.String(), "data-dock-redirect", "the notice stays on the tab so the person reads it")
	assert.Equal(t, movedRoot, fixture.repository.stored.Path, "the saved path is not rolled back")
}

func TestUpdateWithoutPathKeepsTheStoredPath(t *testing.T) {
	fixture := newEditFixture(t)

	rec := postEdit(t, fixture, url.Values{"name": {"Renamed"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `data-dock-redirect="/workspaces"`)
	assert.Equal(t, storedRoot, fixture.repository.stored.Path)
	assert.Equal(t, "Renamed", fixture.repository.stored.Name)
}
