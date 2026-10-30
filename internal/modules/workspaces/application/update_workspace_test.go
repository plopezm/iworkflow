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

// updateFixture is an existing workspace at repoRoot on branch main, with no AGENTS.md installed.
func updateFixture(t *testing.T) (models.Workspace, *fakeRepository, *fakeGit, *fakeAgentsFiles, *application.UpdateWorkspace) {
	t.Helper()
	existing := workspaceAt(t, "Demo", repoRoot, "main")
	repository := newFakeRepository(existing)
	git := newFakeGit()
	agentsFiles := &fakeAgentsFiles{}
	useCase := application.NewUpdateWorkspace(repository, git, agentsFiles)
	return existing, repository, git, agentsFiles, useCase
}

func TestUpdateWorkspaceRenamesAndKeepsBranchAndPath(t *testing.T) {
	existing, repository, git, agentsFiles, useCase := updateFixture(t)
	git.branchErr = werrors.Git("must not be called when the branch is unchanged")

	updated, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "  Nuevo "})

	require.NoError(t, err)
	assert.Equal(t, "Nuevo", updated.Name)
	assert.Equal(t, "main", updated.BaseBranch)
	assert.Equal(t, repoRoot, updated.Path)
	assert.Equal(t, existing.CreatedAt, updated.CreatedAt)
	assert.Equal(t, []models.Workspace{updated}, repository.updated)
	assert.Empty(t, agentsFiles.installs)
}

func TestUpdateWorkspaceRejectsInvalidNameWithoutTouchingAnything(t *testing.T) {
	existing, repository, _, _, useCase := updateFixture(t)

	_, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "   "})

	require.ErrorIs(t, err, werrors.InvalidName())
	assert.Empty(t, repository.updated)
}

func TestUpdateWorkspaceRejectsTooLongName(t *testing.T) {
	existing, repository, _, _, useCase := updateFixture(t)
	tooLong := make([]rune, 81)
	for index := range tooLong {
		tooLong[index] = 'a'
	}

	_, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: string(tooLong)})

	require.ErrorIs(t, err, werrors.InvalidName())
	assert.Empty(t, repository.updated)
}

func TestUpdateWorkspaceAcceptsNameAtCharacterLimit(t *testing.T) {
	existing, _, _, _, useCase := updateFixture(t)
	atLimit := make([]rune, 80)
	for index := range atLimit {
		atLimit[index] = 'ñ'
	}

	_, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: string(atLimit)})

	require.NoError(t, err)
}

func TestUpdateWorkspaceReturnsNotFoundWhenMissing(t *testing.T) {
	_, repository, _, _, useCase := updateFixture(t)
	repository.items = nil
	id, err := models.NewWorkspaceID()
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), id, models.WorkspaceUpdate{Name: "Demo"})

	require.ErrorIs(t, err, werrors.NotFound(id.String()))
}

func TestUpdateWorkspaceChangesBranchWhenItExists(t *testing.T) {
	existing, repository, git, _, useCase := updateFixture(t)
	git.branches[repoRoot+"|develop"] = true

	updated, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "Demo", BaseBranch: " develop "})

	require.NoError(t, err)
	assert.Equal(t, "develop", updated.BaseBranch)
	assert.Equal(t, "develop", repository.items[0].BaseBranch)
}

func TestUpdateWorkspaceRejectsUnknownBranchWithoutSaving(t *testing.T) {
	existing, repository, _, _, useCase := updateFixture(t)

	_, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "Demo", BaseBranch: "nope"})

	require.ErrorIs(t, err, werrors.BranchNotFound("nope"))
	assert.Empty(t, repository.updated)
}

func TestUpdateWorkspaceDoesNotVerifyUnchangedBranch(t *testing.T) {
	existing, repository, git, _, useCase := updateFixture(t)
	git.branchErr = werrors.Git("must not be called")

	_, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "Demo", BaseBranch: "main"})

	require.NoError(t, err)
	assert.Len(t, repository.updated, 1)
}

func TestUpdateWorkspaceSavesThenInstallsAgentsFileWithOverwrite(t *testing.T) {
	existing, repository, _, agentsFiles, useCase := updateFixture(t)

	updated, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "Demo", AgentsFileID: " abc "})

	require.NoError(t, err)
	assert.Equal(t, []installCall{{id: "abc", root: repoRoot, overwrite: true}}, agentsFiles.installs)
	assert.Len(t, repository.updated, 1, "the workspace is saved before the overwrite")
	assert.Equal(t, updated, repository.updated[0])
}

func TestUpdateWorkspaceInstallErrorRestoresOriginalWorkspace(t *testing.T) {
	existing, repository, git, agentsFiles, useCase := updateFixture(t)
	git.branches[repoRoot+"|develop"] = true
	agentsFiles.installErr = werrors.AgentsFileNotFound("zzz")

	_, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "Nuevo", BaseBranch: "develop", AgentsFileID: "zzz"})

	require.ErrorIs(t, err, werrors.AgentsFileNotFound("zzz"))
	assert.EqualError(t, err, "fichero AGENTS.md no encontrado: zzz")
	require.Len(t, repository.updated, 2)
	assert.Equal(t, "Nuevo", repository.updated[0].Name)
	assert.Equal(t, "develop", repository.updated[0].BaseBranch)
	assert.Equal(t, existing, repository.updated[1], "the original workspace is written back")
}

func TestUpdateWorkspaceRevertFailureReportsBothErrors(t *testing.T) {
	existing, repository, _, agentsFiles, useCase := updateFixture(t)
	repository.updateFailFrom = 2
	agentsFiles.installErr = werrors.AgentsFileNotFound("zzz")

	_, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "Nuevo", AgentsFileID: "zzz"})

	require.ErrorIs(t, err, werrors.Repository(""))
	assert.EqualError(t, err,
		"error de persistencia: fichero AGENTS.md no encontrado: zzz; además falló la restauración del workspace: error de persistencia: revert boom")
}

func TestUpdateWorkspacePropagatesRepositoryUpdateErrorsWithoutInstalling(t *testing.T) {
	existing, repository, _, agentsFiles, useCase := updateFixture(t)
	repository.updateErr = werrors.Repository("boom")

	_, err := useCase.Execute(context.Background(), existing.ID, models.WorkspaceUpdate{Name: "Demo", AgentsFileID: "abc"})

	assert.EqualError(t, err, "error de persistencia: boom")
	assert.Empty(t, agentsFiles.installs)
}
