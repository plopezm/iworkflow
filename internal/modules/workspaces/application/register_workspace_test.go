package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/workspaces/application"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
)

const (
	registerInputPath = "/repos/demo/sub"
	repoRoot          = "/repos/demo"
)

func fixedClock() time.Time { return fixedNow }

// registerFixture bundles the fakes a RegisterWorkspace test inspects after running.
type registerFixture struct {
	repository  *fakeRepository
	git         *fakeGit
	agentsFiles *fakeAgentsFiles
	useCase     *application.RegisterWorkspace
}

func newRegisterFixture(t *testing.T) registerFixture {
	t.Helper()
	fixture := registerFixture{
		repository:  newFakeRepository(),
		git:         newFakeGit(),
		agentsFiles: &fakeAgentsFiles{},
	}
	fixture.useCase = application.NewRegisterWorkspace(
		fixture.repository,
		fixture.git,
		fixture.agentsFiles,
		application.Clock(fixedClock),
		application.IDGenerator(newFixedID),
	)
	return fixture
}

func registerInput(branch string) models.NewWorkspace {
	return models.NewWorkspace{Name: "Demo", Path: registerInputPath, BaseBranch: branch, Initialize: true}
}

// gitWithRepo makes the input path a repository whose root is repoRoot.
func gitWithRepo(git *fakeGit, current string, hasCommits bool) {
	git.repos[registerInputPath] = models.GitRepositoryInfo{Root: repoRoot, CurrentBranch: current, HasCommits: hasCommits}
}

func TestRegisterWorkspaceUsesRepositoryRootAndCurrentBranch(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)

	workspace, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	require.NoError(t, err)
	assert.Equal(t, "Demo", workspace.Name)
	assert.Equal(t, repoRoot, workspace.Path)
	assert.Equal(t, "main", workspace.BaseBranch)
	assert.Equal(t, fixedNow, workspace.CreatedAt)
	assert.Equal(t, []models.Workspace{workspace}, fixture.repository.saved)
}

func TestRegisterWorkspaceAssignsIdentifierFromGenerator(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)
	expected, err := models.NewWorkspaceID()
	require.NoError(t, err)
	useCase := application.NewRegisterWorkspace(
		fixture.repository,
		fixture.git,
		fixture.agentsFiles,
		application.Clock(fixedClock),
		application.IDGenerator(func() (models.WorkspaceID, error) { return expected, nil }),
	)

	workspace, err := useCase.Execute(context.Background(), registerInput(""))

	require.NoError(t, err)
	assert.Equal(t, expected, workspace.ID)
}

func TestRegisterWorkspaceFailsWhenIdentifierGenerationFails(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)
	useCase := application.NewRegisterWorkspace(
		fixture.repository,
		fixture.git,
		fixture.agentsFiles,
		application.Clock(fixedClock),
		application.IDGenerator(func() (models.WorkspaceID, error) { return models.WorkspaceID{}, errors.New("rng down") }),
	)

	_, err := useCase.Execute(context.Background(), registerInput(""))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rng down")
	assert.Empty(t, fixture.repository.saved)
}

func TestRegisterWorkspaceUsesExplicitBranchWhenItExists(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)
	fixture.git.branches[repoRoot+"|develop"] = true

	workspace, err := fixture.useCase.Execute(context.Background(), registerInput("develop"))

	require.NoError(t, err)
	assert.Equal(t, "develop", workspace.BaseBranch)
}

func TestRegisterWorkspaceRejectsUnknownBranchWithoutSaving(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)

	_, err := fixture.useCase.Execute(context.Background(), registerInput("nope"))

	require.ErrorIs(t, err, werrors.BranchNotFound("nope"))
	assert.EqualError(t, err, "la rama nope no existe en el repositorio")
	assert.Empty(t, fixture.repository.saved)
}

func TestRegisterWorkspaceRejectsDetachedHeadWithoutExplicitBranch(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "", true)

	_, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	require.ErrorIs(t, err, werrors.NoCurrentBranch())
}

func TestRegisterWorkspaceRejectsAlreadyRegisteredRoot(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)
	fixture.repository.items = append(fixture.repository.items, workspaceAt(t, "Existing", repoRoot, "main"))

	_, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	require.ErrorIs(t, err, werrors.AlreadyRegistered(repoRoot))
	assert.EqualError(t, err, "ya existe un workspace para la ruta "+repoRoot)
}

func TestRegisterWorkspacePropagatesGitErrorsWithoutSaving(t *testing.T) {
	fixture := newRegisterFixture(t)
	fixture.git.inspectErr = werrors.Git("boom")

	_, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	require.ErrorIs(t, err, werrors.Git(""))
	assert.Empty(t, fixture.repository.saved)
	assert.Empty(t, fixture.git.inits)
}

func TestRegisterWorkspaceCreatesMissingFolderByInitializingRepository(t *testing.T) {
	fixture := newRegisterFixture(t)

	workspace, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	require.NoError(t, err)
	assert.Equal(t, []string{registerInputPath}, fixture.git.inits)
	assert.Equal(t, []string{"main"}, fixture.git.initBranches, "without a branch the default is main")
	assert.Equal(t, "main", workspace.BaseBranch)
	assert.Equal(t, []string{registerInputPath}, fixture.git.commits, "an unborn repository gets its initial commit")
	assert.Len(t, fixture.repository.saved, 1)
}

func TestRegisterWorkspaceInitializesNonGitFolderOnExplicitBranch(t *testing.T) {
	fixture := newRegisterFixture(t)
	fixture.git.plainFolders[registerInputPath] = true
	fixture.git.branches[registerInputPath+"|develop"] = true

	workspace, err := fixture.useCase.Execute(context.Background(), registerInput("develop"))

	require.NoError(t, err)
	assert.Equal(t, []string{"develop"}, fixture.git.initBranches)
	assert.Equal(t, "develop", workspace.BaseBranch)
}

func TestRegisterWorkspaceCreatesInitialCommitWhenRepositoryHasNoCommits(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", false)

	workspace, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	require.NoError(t, err)
	assert.Equal(t, []string{repoRoot}, fixture.git.commits, "the commit runs on the repository root")
	assert.Equal(t, "main", workspace.BaseBranch)
}

func TestRegisterWorkspaceDoesNotInitOrCommitWhenAlreadyRegistered(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", false)
	fixture.repository.items = append(fixture.repository.items, workspaceAt(t, "Existing", repoRoot, "main"))

	_, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	require.ErrorIs(t, err, werrors.AlreadyRegistered(repoRoot))
	assert.Empty(t, fixture.git.commits)
	assert.Empty(t, fixture.git.inits)
}

func TestRegisterWorkspaceRequiresConfirmationForMissingFolderWithoutMutating(t *testing.T) {
	fixture := newRegisterFixture(t)
	input := registerInput("")
	input.Initialize = false

	_, err := fixture.useCase.Execute(context.Background(), input)

	require.ErrorIs(t, err, werrors.InitializationRequired("", werrors.ReasonMissingFolder))
	path, reason, ok := werrors.AsInitializationRequired(err)
	require.True(t, ok)
	assert.Equal(t, registerInputPath, path)
	assert.Equal(t, werrors.ReasonMissingFolder, reason)
	assert.Empty(t, fixture.git.inits)
	assert.Empty(t, fixture.repository.saved)
}

func TestRegisterWorkspaceRequiresConfirmationForNonGitFolderWithoutMutating(t *testing.T) {
	fixture := newRegisterFixture(t)
	fixture.git.plainFolders[registerInputPath] = true
	input := registerInput("")
	input.Initialize = false

	_, err := fixture.useCase.Execute(context.Background(), input)

	path, reason, ok := werrors.AsInitializationRequired(err)
	require.True(t, ok, "unexpected error: %v", err)
	assert.Equal(t, registerInputPath, path)
	assert.Equal(t, werrors.ReasonNotAGitRepository, reason)
	assert.Empty(t, fixture.git.inits)
}

func TestRegisterWorkspaceRequiresConfirmationForMissingCommitsWithoutCommitting(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", false)
	input := registerInput("")
	input.Initialize = false

	_, err := fixture.useCase.Execute(context.Background(), input)

	path, reason, ok := werrors.AsInitializationRequired(err)
	require.True(t, ok, "unexpected error: %v", err)
	assert.Equal(t, repoRoot, path)
	assert.Equal(t, werrors.ReasonNoCommits, reason)
	assert.Empty(t, fixture.git.commits)
}

func TestRegisterWorkspaceInitErrorsPropagateWithoutSaving(t *testing.T) {
	fixture := newRegisterFixture(t)
	fixture.git.initErr = werrors.Git("init failed")

	_, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	assert.EqualError(t, err, "error de git: init failed")
	assert.Empty(t, fixture.repository.saved)
}

func TestRegisterWorkspaceCommitErrorsPropagateWithoutSaving(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", false)
	fixture.git.commitErr = werrors.Git("commit failed")

	_, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	assert.EqualError(t, err, "error de git: commit failed")
	assert.Empty(t, fixture.repository.saved)
}

func TestRegisterWorkspaceInstallsAgentsFileIntoRootWithoutOverwrite(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)
	input := registerInput("")
	input.AgentsFileID = " abc "

	_, err := fixture.useCase.Execute(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, []installCall{{id: "abc", root: repoRoot, overwrite: false}}, fixture.agentsFiles.installs)
	assert.Len(t, fixture.repository.saved, 1)
}

func TestRegisterWorkspaceInstallErrorAbortsWithoutSaving(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)
	fixture.agentsFiles.installErr = werrors.AgentsFileTargetExists(repoRoot + "/AGENTS.md")
	input := registerInput("")
	input.AgentsFileID = "abc"

	_, err := fixture.useCase.Execute(context.Background(), input)

	assert.EqualError(t, err, "ya existe AGENTS.md en /repos/demo/AGENTS.md")
	assert.Empty(t, fixture.repository.saved)
}

func TestRegisterWorkspaceInstallsAgentsFileAfterInitialCommit(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", false)
	fixture.agentsFiles.installErr = werrors.AgentsFile("disk full")
	input := registerInput("")
	input.AgentsFileID = "abc"

	_, err := fixture.useCase.Execute(context.Background(), input)

	require.ErrorIs(t, err, werrors.AgentsFile(""))
	assert.Len(t, fixture.git.commits, 1, "the commit happens before the install that fails")
	assert.Empty(t, fixture.repository.saved)
}

func TestRegisterWorkspaceDoesNotTouchAgentsFilesWhenNotRequested(t *testing.T) {
	fixture := newRegisterFixture(t)
	gitWithRepo(fixture.git, "main", true)

	_, err := fixture.useCase.Execute(context.Background(), registerInput(""))

	require.NoError(t, err)
	assert.Empty(t, fixture.agentsFiles.installs)
}

func TestRegisterWorkspaceRejectsInvalidInputBeforeAnyGitCall(t *testing.T) {
	cases := map[string]models.NewWorkspace{
		"blank name":    {Name: "   ", Path: registerInputPath, Initialize: true},
		"relative path": {Name: "Demo", Path: "relative/dir", Initialize: true},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newRegisterFixture(t)

			_, err := fixture.useCase.Execute(context.Background(), input)

			require.Error(t, err)
			assert.Empty(t, fixture.git.inits)
			assert.Empty(t, fixture.repository.saved)
		})
	}
}
