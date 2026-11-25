package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
)

func TestParseRefReadsTheKindAndKeepsTheRemoteInTheName(t *testing.T) {
	assert.Equal(t, Ref{Kind: KindLocal, Name: "feature/x"}, ParseRef("local:feature/x"))
	assert.Equal(t, Ref{Kind: KindRemote, Name: "origin/feature/x"}, ParseRef("remote:origin/feature/x"))
}

func TestParseRefKeepsAMalformedValueSoItIsRejected(t *testing.T) {
	for _, value := range []string{"feature/x", "other:x", ""} {
		ref := ParseRef(value)

		assert.Equal(t, Kind(""), ref.Kind, value)
		assert.ErrorIs(t, ValidateRef(ref), werrors.ErrInvalidBranch, value)
	}
}

func TestRefStringIsTheFormValue(t *testing.T) {
	assert.Equal(t, "local:develop", Ref{Kind: KindLocal, Name: "develop"}.String())
	assert.Equal(t, "remote:origin/develop", Ref{Kind: KindRemote, Name: "origin/develop"}.String())
}

func TestValidateRefRejectsWhatGitMustNeverReadAsAnOption(t *testing.T) {
	invalid := []Ref{
		{Kind: KindLocal, Name: ""},
		{Kind: KindLocal, Name: "-D"},
		{Kind: KindLocal, Name: "a\x00b"},
		{Kind: KindRemote, Name: "origin/"},
		{Kind: KindRemote, Name: "/develop"},
		{Kind: KindRemote, Name: "origin/-force"},
		{Kind: KindRemote, Name: "-origin/develop"},
	}
	for _, ref := range invalid {
		assert.ErrorIs(t, ValidateRef(ref), werrors.ErrInvalidBranch, ref.String())
	}
	require.NoError(t, ValidateRef(Ref{Kind: KindLocal, Name: "feature/x"}))
	require.NoError(t, ValidateRef(Ref{Kind: KindRemote, Name: "origin/feature/x"}))
}

func TestProtectGivesTheFirstApplyingReason(t *testing.T) {
	guard := Guard{BaseBranch: "develop", TaskBranches: []string{"iwf/task"}}

	// A branch that is current, the base and a worktree checkout reports the current-branch reason.
	current := Protect(Branch{Name: "develop", Current: true, CheckedOut: true}, guard)
	assert.Equal(t, ReasonCheckedOut, current.Reason)
	assert.True(t, current.Protected)

	assert.Equal(t, ReasonWorktree, Protect(Branch{Name: "wip", CheckedOut: true}, guard).Reason)
	assert.Equal(t, ReasonTaskInUse, Protect(Branch{Name: "iwf/task"}, guard).Reason)
	assert.Equal(t, ReasonRemoteHead, Protect(Branch{Name: "HEAD", Remote: true, RemoteName: "origin"}, guard).Reason)
}

func TestProtectLeavesAnOrdinaryBranchDeletable(t *testing.T) {
	branch := Protect(Branch{Name: "feature/x"}, Guard{BaseBranch: "develop"})

	assert.False(t, branch.Protected)
	assert.Equal(t, ReasonNone, branch.Reason)
}

func TestProtectWithoutBaseBranchDoesNotProtectEmptyNames(t *testing.T) {
	branch := Protect(Branch{Name: "feature/x"}, Guard{})

	assert.False(t, branch.Protected)
}

func TestBranchRefMatchesTheRemoteFullName(t *testing.T) {
	remote := Branch{Name: "develop", Remote: true, RemoteName: "origin"}

	assert.Equal(t, "origin/develop", remote.FullName())
	assert.True(t, remote.Matches(Ref{Kind: KindRemote, Name: "origin/develop"}))
	assert.False(t, remote.Matches(Ref{Kind: KindLocal, Name: "develop"}))
}

func TestProtectRemoteFormBaseProtectsTheLocalAndTheRemoteBranch(t *testing.T) {
	guard := Guard{BaseBranch: "origin/develop", Remotes: []string{"origin"}}

	local := Protect(Branch{Name: "develop"}, guard)
	assert.True(t, local.Protected, "local develop is the base of origin/develop")
	assert.Equal(t, ReasonBaseBranch, local.Reason)

	remote := Protect(Branch{Name: "develop", Remote: true, RemoteName: "origin"}, guard)
	assert.True(t, remote.Protected)
	assert.Equal(t, ReasonBaseBranch, remote.Reason)

	assert.False(t, Protect(Branch{Name: "feature/old"}, guard).Protected)
}

func TestProtectRemoteFormBaseWithoutListedRemoteStillProtectsItsFullName(t *testing.T) {
	guard := Guard{BaseBranch: "origin/develop"}

	remote := Protect(Branch{Name: "develop", Remote: true, RemoteName: "origin"}, guard)
	assert.True(t, remote.Protected, "the full name origin/develop is the base")
	assert.Equal(t, ReasonBaseBranch, remote.Reason)
}

func TestProtectPlainBaseWithASlashProtectsOnlyItsNameAndRemoteNamesake(t *testing.T) {
	// "feature" is not a listed remote, so "feature/x" is a plain local name.
	guard := Guard{BaseBranch: "feature/x", Remotes: []string{"origin"}}

	assert.True(t, Protect(Branch{Name: "feature/x"}, guard).Protected)
	assert.True(t, Protect(Branch{Name: "feature/x", Remote: true, RemoteName: "origin"}, guard).Protected)
	assert.False(t, Protect(Branch{Name: "x"}, guard).Protected, "the slash does not make feature a remote")
	assert.False(t, Protect(Branch{Name: "x", Remote: true, RemoteName: "origin"}, guard).Protected)
}

func TestProtectRemoteFormBaseUsesTheLongestListedRemote(t *testing.T) {
	// Remotes "a" and "a/b" are both listed: "a/b/develop" is branch develop of remote a/b, not b/develop of a.
	guard := Guard{BaseBranch: "a/b/develop", Remotes: []string{"a", "a/b"}}

	assert.True(t, Protect(Branch{Name: "develop"}, guard).Protected)
	assert.True(t, Protect(Branch{Name: "develop", Remote: true, RemoteName: "a/b"}, guard).Protected)
	assert.False(t, Protect(Branch{Name: "b/develop"}, guard).Protected)
}
