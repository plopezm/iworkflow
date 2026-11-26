package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/workspaces/application"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
)

// relocatedRoot is where the repository of the workspace was moved to, and relocatedSub a folder inside it.
const (
	relocatedRoot = "/moved/repo"
	relocatedSub  = "/moved/repo/sub"
)

// pathFixture is a workspace at repoRoot, whose repository has the root commit "first". The same
// repository was moved to relocatedRoot, so its root commit is the same. Tests break one rule at a time.
type pathFixture struct {
	existing   models.Workspace
	repository *fakeRepository
	git        *fakeGit
	tasks      *fakeWorkspaceTasks
	agents     *fakeAgentsFiles
	useCase    *application.UpdateWorkspace
}

func newPathFixture(t *testing.T) pathFixture {
	t.Helper()
	existing := workspaceAt(t, "Demo", repoRoot, "main")
	repository := newFakeRepository(existing)
	git := newFakeGit()
	git.roots[repoRoot] = []string{"first"}
	git.repos[relocatedRoot] = models.GitRepositoryInfo{Root: relocatedRoot, CurrentBranch: "main", HasCommits: true}
	git.roots[relocatedRoot] = []string{"first"}
	tasks := &fakeWorkspaceTasks{}
	agents := &fakeAgentsFiles{}
	useCase := application.NewUpdateWorkspace(repository, git, agents, &fakeRuntimes{installed: []string{"pi"}}, tasks)
	return pathFixture{existing: existing, repository: repository, git: git, tasks: tasks, agents: agents, useCase: useCase}
}

func TestUpdateWorkspacePathRejectionsSaveNothing(t *testing.T) {
	tests := []struct {
		name    string
		newPath string
		mutate  func(f *pathFixture)
		wantErr error
	}{
		{
			name:    "relative path",
			newPath: "relative/repo",
			wantErr: werrors.PathNotAbsolute("relative/repo"),
		},
		{
			name:    "unfinished tasks block the change",
			newPath: relocatedRoot,
			mutate:  func(f *pathFixture) { f.tasks.blocking = true },
			wantErr: werrors.PathBlockedByTasks(),
		},
		{
			name:    "missing folder",
			newPath: "/nowhere/repo",
			wantErr: werrors.PathNotFound("/nowhere/repo"),
		},
		{
			name:    "plain folder",
			newPath: "/plain/folder",
			mutate:  func(f *pathFixture) { f.git.plainFolders["/plain/folder"] = true },
			wantErr: werrors.NotAGitRepository("/plain/folder"),
		},
		{
			name:    "subfolder of a repository",
			newPath: relocatedSub,
			mutate: func(f *pathFixture) {
				f.git.repos[relocatedSub] = models.GitRepositoryInfo{Root: relocatedRoot, CurrentBranch: "main", HasCommits: true}
			},
			wantErr: werrors.PathNotRepositoryRoot(relocatedSub),
		},
		{
			name:    "another project",
			newPath: relocatedRoot,
			mutate:  func(f *pathFixture) { f.git.roots[relocatedRoot] = []string{"other"} },
			wantErr: werrors.PathNotSameProject(relocatedRoot),
		},
		{
			name:    "new repository without commits",
			newPath: relocatedRoot,
			mutate:  func(f *pathFixture) { f.git.roots[relocatedRoot] = nil },
			wantErr: werrors.PathNotSameProject(relocatedRoot),
		},
		{
			name:    "old repository without commits",
			newPath: relocatedRoot,
			mutate:  func(f *pathFixture) { f.git.roots[repoRoot] = nil },
			wantErr: werrors.PathNotSameProject(relocatedRoot),
		},
		{
			name:    "old repository cannot be read",
			newPath: relocatedRoot,
			mutate:  func(f *pathFixture) { f.git.rootsErr = werrors.Git("unreadable") },
			wantErr: werrors.Git(""),
		},
		{
			name:    "path already used by another workspace",
			newPath: relocatedRoot,
			mutate: func(f *pathFixture) {
				other := workspaceAt(t, "Other", relocatedRoot, "main")
				f.repository.items = append(f.repository.items, other)
			},
			wantErr: werrors.AlreadyRegistered(relocatedRoot),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newPathFixture(t)
			if tc.mutate != nil {
				tc.mutate(&f)
			}

			_, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: tc.newPath})

			require.ErrorIs(t, err, tc.wantErr)
			assert.Empty(t, f.repository.updated, "a rejected path change saves nothing")
			assert.Empty(t, f.git.repaired, "a rejected path change repairs nothing")
			assert.Equal(t, repoRoot, f.repository.items[0].Path, "the stored path is unchanged")
		})
	}
}

func TestUpdateWorkspaceMovesThePathToTheSameProjectAndSavesIt(t *testing.T) {
	f := newPathFixture(t)

	updated, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: relocatedRoot})

	require.NoError(t, err)
	assert.Equal(t, relocatedRoot, updated.Path)
	assert.Equal(t, []models.Workspace{updated}, f.repository.updated)
	assert.Empty(t, f.git.repaired, "without linked worktrees there is nothing to repair")
}

func TestUpdateWorkspaceAcceptsAMovedFolderWhenTheOldOneIsGone(t *testing.T) {
	f := newPathFixture(t)
	// The old folder no longer exists, so its root commits cannot be read. This is a deliberate decision: the
	// new path is accepted on the strength of the repository check alone (see UpdateWorkspace.checkSameProject).
	delete(f.git.roots, repoRoot)

	updated, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: relocatedRoot})

	require.NoError(t, err)
	assert.Equal(t, relocatedRoot, updated.Path)
}

func TestUpdateWorkspaceSamePathSkipsThePathChecks(t *testing.T) {
	f := newPathFixture(t)
	f.tasks.blockingErr = werrors.Git("must not be called when the path is unchanged")
	f.git.rootsErr = werrors.Git("must not be called when the path is unchanged")

	updated, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: repoRoot})

	require.NoError(t, err)
	assert.Equal(t, repoRoot, updated.Path)
}

func TestUpdateWorkspaceRepairsTheWorktreesAfterSavingWhenTheyExist(t *testing.T) {
	f := newPathFixture(t)
	f.git.linked[relocatedRoot] = true
	savedBeforeRepair := -1
	f.git.repairHook = func() { savedBeforeRepair = len(f.repository.updated) }

	updated, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: relocatedRoot})

	require.NoError(t, err)
	assert.Equal(t, relocatedRoot, updated.Path)
	assert.Equal(t, []string{relocatedRoot}, f.git.repaired)
	assert.Equal(t, 1, savedBeforeRepair, "the path is saved before the repair runs")
}

func TestUpdateWorkspaceReportsARepairFailureButKeepsThePathSaved(t *testing.T) {
	f := newPathFixture(t)
	f.git.linked[relocatedRoot] = true
	f.git.repairErr = werrors.Git("worktree broke")

	updated, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: relocatedRoot})

	require.ErrorIs(t, err, werrors.WorktreeRepair(""))
	assert.Equal(t, relocatedRoot, updated.Path, "the updated workspace is still returned")
	require.Len(t, f.repository.updated, 1)
	assert.Equal(t, relocatedRoot, f.repository.items[0].Path, "the saved path is not rolled back")
}

func TestUpdateWorkspaceRevertsThePathWhenTheAgentsFileFails(t *testing.T) {
	f := newPathFixture(t)
	f.agents.installErr = werrors.AgentsFileNotFound("zzz")

	_, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: relocatedRoot, AgentsFileID: "zzz"})

	require.ErrorIs(t, err, werrors.AgentsFileNotFound("zzz"))
	require.Len(t, f.repository.updated, 2)
	assert.Equal(t, relocatedRoot, f.repository.updated[0].Path)
	assert.Equal(t, repoRoot, f.repository.updated[1].Path, "the original path is written back")
	assert.Empty(t, f.git.repaired, "nothing is repaired when the change is reverted")
}

func TestUpdateWorkspaceChecksANewBranchInTheNewRoot(t *testing.T) {
	f := newPathFixture(t)
	f.git.branches[relocatedRoot+"|develop"] = true

	updated, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: relocatedRoot, BaseBranch: "develop"})

	require.NoError(t, err)
	assert.Equal(t, relocatedRoot, updated.Path)
	assert.Equal(t, "develop", updated.BaseBranch)
}

func TestUpdateWorkspaceRejectsABranchOnlyTheOldRootHasWithoutSaving(t *testing.T) {
	f := newPathFixture(t)
	f.git.branches[repoRoot+"|develop"] = true

	_, err := f.useCase.Execute(context.Background(), f.existing.ID, models.WorkspaceUpdate{Name: "Demo", Path: relocatedRoot, BaseBranch: "develop"})

	require.ErrorIs(t, err, werrors.BranchNotFound("develop"))
	assert.Empty(t, f.repository.updated)
}

func TestPathEditBlockerLocksThePathWhileTasksAreUnfinished(t *testing.T) {
	tests := []struct {
		name        string
		blocking    bool
		blockingErr error
		wantLocked  bool
		wantErr     error
	}{
		{name: "unfinished tasks lock the path", blocking: true, wantLocked: true},
		{name: "no unfinished task leaves it editable", blocking: false, wantLocked: false},
		{name: "a failing lookup is reported", blockingErr: werrors.Git("boom"), wantErr: werrors.Git("")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newPathFixture(t)
			f.tasks.blocking = tc.blocking
			f.tasks.blockingErr = tc.blockingErr

			locked, err := f.useCase.PathEditBlocker(context.Background(), f.existing.ID)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantLocked, locked)
			assert.Empty(t, f.repository.updated, "the read does not save anything")
		})
	}
}
