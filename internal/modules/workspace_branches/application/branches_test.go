package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	"iworkflow/internal/modules/workspace_branches/domain/models"
	"iworkflow/internal/modules/workspace_branches/domain/ports/outbound"
)

const workspaceID = "ws-1"

func newWorkspaces() *fakeWorkspaces {
	return &fakeWorkspaces{byID: map[string]outbound.WorkspaceTarget{
		workspaceID: {ID: workspaceID, Name: "alpha", Path: "/repos/alpha", BaseBranch: "develop"},
	}}
}

// sampleGit has the usual shapes: the current branch, the base branch, a worktree branch, a live task branch,
// a plain local branch, a remote-tracking branch and the remote default ref.
func sampleGit() *fakeGit {
	return &fakeGit{remotes: []string{"origin"}, branches: []models.Branch{
		{Name: "develop"},
		{Name: "main", Current: true, CheckedOut: true},
		{Name: "feature/old"},
		{Name: "feature/wt", CheckedOut: true},
		{Name: "iwf/live-task"},
		{Name: "gone", Remote: true, RemoteName: "origin"},
		{Name: "HEAD", Remote: true, RemoteName: "origin"},
		{Name: "develop", Remote: true, RemoteName: "origin"},
	}}
}

func newLister(git *fakeGit, tasks *fakeTasks) *ListBranches {
	return NewListBranches(newWorkspaces(), git, tasks)
}

func newDeleter(git *fakeGit, tasks *fakeTasks) *DeleteBranches {
	return NewDeleteBranches(newWorkspaces(), git, tasks)
}

func branchNamed(t *testing.T, list BranchList, fullName string) models.Branch {
	t.Helper()
	for _, branch := range list.Branches {
		if branch.FullName() == fullName {
			return branch
		}
	}
	t.Fatalf("branch %q not listed", fullName)
	return models.Branch{}
}

func TestListBranchesProtectsEveryS5Case(t *testing.T) {
	git := sampleGit()
	list, err := newLister(git, &fakeTasks{names: []string{"iwf/live-task"}}).Execute(context.Background(), workspaceID, "")
	require.NoError(t, err)

	cases := map[string]models.ProtectionReason{
		"main":           models.ReasonCheckedOut,
		"develop":        models.ReasonBaseBranch,
		"feature/wt":     models.ReasonWorktree,
		"iwf/live-task":  models.ReasonTaskInUse,
		"origin/develop": models.ReasonBaseBranch,
		"origin/HEAD":    models.ReasonRemoteHead,
		"feature/old":    models.ReasonNone,
		"origin/gone":    models.ReasonNone,
	}
	for name, reason := range cases {
		branch := branchNamed(t, list, name)
		assert.Equal(t, reason, branch.Reason, name)
		assert.Equal(t, reason != models.ReasonNone, branch.Protected, name)
	}
}

func TestListBranchesFiltersByNameOnTheServerIgnoringCase(t *testing.T) {
	list, err := newLister(sampleGit(), &fakeTasks{}).Execute(context.Background(), workspaceID, "  FEATURE ")
	require.NoError(t, err)

	require.Len(t, list.Branches, 2)
	assert.Equal(t, "feature/old", list.Branches[0].FullName())
	assert.Equal(t, "feature/wt", list.Branches[1].FullName())
	assert.Equal(t, "FEATURE", list.Filter)
}

func TestListBranchesWithoutFilterShowsLocalThenRemote(t *testing.T) {
	list, err := newLister(sampleGit(), &fakeTasks{}).Execute(context.Background(), workspaceID, "")
	require.NoError(t, err)

	require.Len(t, list.Branches, 8)
	assert.False(t, list.Branches[0].Remote)
	assert.True(t, list.Branches[len(list.Branches)-1].Remote)
}

func TestListBranchesFailsForAnUnknownWorkspace(t *testing.T) {
	_, err := newLister(sampleGit(), &fakeTasks{}).Execute(context.Background(), "missing", "")

	require.Error(t, err)
	assert.True(t, errors.Is(err, werrors.ErrWorkspaceNotFound))
}

func TestListBranchesReportsAGitFailure(t *testing.T) {
	git := &fakeGit{listErr: errGitRefused}
	_, err := newLister(git, &fakeTasks{}).Execute(context.Background(), workspaceID, "")

	assert.ErrorIs(t, err, errGitRefused)
}

func TestDeleteBranchesDeletesLocalAndRemoteBranches(t *testing.T) {
	git := sampleGit()
	results, err := newDeleter(git, &fakeTasks{}).Execute(context.Background(), workspaceID, []models.Ref{
		{Kind: models.KindLocal, Name: "feature/old"},
		{Kind: models.KindRemote, Name: "origin/gone"},
	})

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, OutcomeDeleted, results[0].Outcome)
	assert.Equal(t, OutcomeDeleted, results[1].Outcome)
	assert.Equal(t, []string{
		"delete-local /repos/alpha feature/old",
		"delete-remote /repos/alpha origin gone",
	}, git.deletions())
}

func TestDeleteBranchesRefusesProtectedBranchesWithoutCallingGit(t *testing.T) {
	git := sampleGit()
	refs := []models.Ref{
		{Kind: models.KindLocal, Name: "main"},
		{Kind: models.KindLocal, Name: "develop"},
		{Kind: models.KindLocal, Name: "feature/wt"},
		{Kind: models.KindLocal, Name: "iwf/live-task"},
		{Kind: models.KindRemote, Name: "origin/HEAD"},
	}
	results, err := newDeleter(git, &fakeTasks{names: []string{"iwf/live-task"}}).Execute(context.Background(), workspaceID, refs)

	require.NoError(t, err)
	require.Len(t, results, len(refs))
	for _, result := range results {
		assert.Equal(t, OutcomeRefused, result.Outcome, result.Ref.String())
	}
	assert.Equal(t, models.ReasonCheckedOut, results[0].Reason)
	assert.Equal(t, models.ReasonBaseBranch, results[1].Reason)
	assert.Equal(t, models.ReasonWorktree, results[2].Reason)
	assert.Equal(t, models.ReasonTaskInUse, results[3].Reason)
	assert.Equal(t, models.ReasonRemoteHead, results[4].Reason)
	assert.Empty(t, git.deletions(), "a refused branch never reaches git")
}

func TestDeleteBranchesNeverPassesAnOptionToGit(t *testing.T) {
	cases := []models.Ref{
		{Kind: models.KindLocal, Name: "-D"},
		{Kind: models.KindLocal, Name: "--force"},
		{Kind: models.KindRemote, Name: "origin/-x"},
		{Kind: models.KindRemote, Name: "-origin/x"},
		{Kind: "", Name: "-x"},
		{Kind: models.KindLocal, Name: ""},
		{Kind: models.KindLocal, Name: "a\x00b"},
		{Kind: models.KindRemote, Name: "originonly"},
	}
	for _, ref := range cases {
		git := sampleGit()
		git.branches = append(git.branches, models.Branch{Name: "-x"})
		results, err := newDeleter(git, &fakeTasks{}).Execute(context.Background(), workspaceID, []models.Ref{ref})

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, OutcomeFailed, results[0].Outcome, ref.String())
		assert.ErrorIs(t, results[0].Err, werrors.ErrInvalidBranch, ref.String())
		assert.Empty(t, git.deletions(), ref.String())
	}
}

func TestDeleteBranchesOnlyDeletesRemoteRefsThatExistAsRemoteTracking(t *testing.T) {
	git := sampleGit()
	results, err := newDeleter(git, &fakeTasks{}).Execute(context.Background(), workspaceID, []models.Ref{
		{Kind: models.KindRemote, Name: "origin/never-fetched"},
		{Kind: models.KindLocal, Name: "never-created"},
	})

	require.NoError(t, err)
	assert.ErrorIs(t, results[0].Err, werrors.ErrBranchNotFound)
	assert.ErrorIs(t, results[1].Err, werrors.ErrBranchNotFound)
	assert.Empty(t, git.deletions())
}

func TestDeleteBranchesOneFailureDoesNotAbortTheRest(t *testing.T) {
	git := sampleGit()
	git.failLocal = map[string]error{"feature/old": errGitRefused}
	results, err := newDeleter(git, &fakeTasks{}).Execute(context.Background(), workspaceID, []models.Ref{
		{Kind: models.KindLocal, Name: "feature/old"},
		{Kind: models.KindRemote, Name: "origin/gone"},
	})

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, OutcomeFailed, results[0].Outcome)
	assert.ErrorIs(t, results[0].Err, errGitRefused)
	assert.Equal(t, OutcomeDeleted, results[1].Outcome)
	assert.Len(t, git.deletions(), 2)
}

func newRemoteFormBaseWorkspaces(base string) *fakeWorkspaces {
	return &fakeWorkspaces{byID: map[string]outbound.WorkspaceTarget{
		workspaceID: {ID: workspaceID, Name: "alpha", Path: "/repos/alpha", BaseBranch: base},
	}}
}

func TestListBranchesProtectsTheLocalAndRemoteBranchOfARemoteFormBase(t *testing.T) {
	list, err := NewListBranches(newRemoteFormBaseWorkspaces("origin/develop"), sampleGit(), &fakeTasks{}).
		Execute(context.Background(), workspaceID, "")
	require.NoError(t, err)

	assert.Equal(t, models.ReasonBaseBranch, branchNamed(t, list, "develop").Reason, "local develop")
	assert.Equal(t, models.ReasonBaseBranch, branchNamed(t, list, "origin/develop").Reason, "origin/develop")
	assert.Equal(t, models.ReasonNone, branchNamed(t, list, "feature/old").Reason)
}

func TestDeleteBranchesRefusesBothBranchesOfARemoteFormBase(t *testing.T) {
	git := sampleGit()
	results, err := NewDeleteBranches(newRemoteFormBaseWorkspaces("origin/develop"), git, &fakeTasks{}).
		Execute(context.Background(), workspaceID, []models.Ref{
			{Kind: models.KindLocal, Name: "develop"},
			{Kind: models.KindRemote, Name: "origin/develop"},
		})

	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, result := range results {
		assert.Equal(t, OutcomeRefused, result.Outcome, result.Ref.String())
		assert.Equal(t, models.ReasonBaseBranch, result.Reason, result.Ref.String())
	}
	assert.Empty(t, git.deletions(), "a protected base never reaches git")
}

func TestListBranchesPlainSlashedBaseProtectsOnlyItsNameAndRemoteNamesake(t *testing.T) {
	git := &fakeGit{remotes: []string{"origin"}, branches: []models.Branch{
		{Name: "feature/x"},
		{Name: "feature/x", Remote: true, RemoteName: "origin"},
		{Name: "x"},
		{Name: "x", Remote: true, RemoteName: "origin"},
	}}
	list, err := NewListBranches(newRemoteFormBaseWorkspaces("feature/x"), git, &fakeTasks{}).
		Execute(context.Background(), workspaceID, "")
	require.NoError(t, err)

	assert.Equal(t, models.ReasonBaseBranch, branchNamed(t, list, "feature/x").Reason)
	assert.Equal(t, models.ReasonBaseBranch, branchNamed(t, list, "origin/feature/x").Reason)
	assert.False(t, branchNamed(t, list, "x").Protected)
	assert.False(t, branchNamed(t, list, "origin/x").Protected)
}

func TestDeleteBranchesFailsForAnUnknownWorkspace(t *testing.T) {
	_, err := newDeleter(sampleGit(), &fakeTasks{}).Execute(context.Background(), "missing", []models.Ref{{Kind: models.KindLocal, Name: "x"}})

	assert.True(t, errors.Is(err, werrors.ErrWorkspaceNotFound))
}

func TestDeleteBranchesStopsWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	git := sampleGit()
	results, err := newDeleter(git, &fakeTasks{}).Execute(ctx, workspaceID, []models.Ref{{Kind: models.KindLocal, Name: "feature/old"}})

	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, results)
	assert.Empty(t, git.deletions())
}

func TestDeleteBranchesRefusesARefThatTwoListedBranchesMatch(t *testing.T) {
	// Remote "a" with branch "b/c" and remote "a/b" with branch "c" both have the reference a/b/c.
	git := &fakeGit{remotes: []string{"a", "a/b"}, branches: []models.Branch{
		{Name: "b/c", Remote: true, RemoteName: "a"},
		{Name: "c", Remote: true, RemoteName: "a/b"},
	}}
	results, err := newDeleter(git, &fakeTasks{}).Execute(context.Background(), workspaceID, []models.Ref{
		{Kind: models.KindRemote, Name: "a/b/c"},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, OutcomeFailed, results[0].Outcome)
	assert.ErrorIs(t, results[0].Err, werrors.ErrAmbiguousBranch)
	assert.Empty(t, git.deletions(), "an ambiguous reference deletes nothing")
}

func TestDeleteBranchesHandlesARepeatedReferenceOnce(t *testing.T) {
	git := sampleGit()
	ref := models.Ref{Kind: models.KindLocal, Name: "feature/old"}
	results, err := newDeleter(git, &fakeTasks{}).Execute(context.Background(), workspaceID, []models.Ref{ref, ref})

	require.NoError(t, err)
	require.Len(t, results, 1, "a repeated reference gets one outcome")
	assert.Equal(t, OutcomeDeleted, results[0].Outcome)
	assert.Equal(t, []string{"delete-local /repos/alpha feature/old"}, git.deletions())
}

func TestDeleteBranchesAbortsWithoutDeletingWhenTheTaskLookupFails(t *testing.T) {
	git := sampleGit()
	results, err := newDeleter(git, &fakeTasks{err: errGitRefused}).Execute(context.Background(), workspaceID, []models.Ref{
		{Kind: models.KindLocal, Name: "feature/old"},
		{Kind: models.KindRemote, Name: "origin/gone"},
	})

	assert.ErrorIs(t, err, errGitRefused)
	assert.Empty(t, results)
	assert.Empty(t, git.deletions(), "the batch aborts before any deletion")
}

func TestDeleteBranchesAbortsWithoutDeletingWhenListingRemotesFails(t *testing.T) {
	git := sampleGit()
	git.remotesErr = errGitRefused
	results, err := newDeleter(git, &fakeTasks{}).Execute(context.Background(), workspaceID, []models.Ref{
		{Kind: models.KindLocal, Name: "feature/old"},
		{Kind: models.KindRemote, Name: "origin/gone"},
	})

	assert.ErrorIs(t, err, errGitRefused)
	assert.Empty(t, results)
	assert.Empty(t, git.deletions(), "the batch aborts before any deletion")
}
