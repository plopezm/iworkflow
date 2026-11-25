// Package models holds the branch model of the workspace_branches module: what a branch is, why it may
// not be deleted, and how a branch is referenced from a request.
package models

import (
	"fmt"
	"sort"
	"strings"

	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
)

// Kind tells a local branch from a remote-tracking one.
type Kind string

// Branch kinds, as they appear in a branch reference ("local:name", "remote:origin/name").
const (
	KindLocal  Kind = "local"
	KindRemote Kind = "remote"
)

// ProtectionReason says why a branch cannot be deleted. The empty reason means it can.
type ProtectionReason string

// Protection reasons (S5 of the spec).
const (
	ReasonNone       ProtectionReason = ""
	ReasonCheckedOut ProtectionReason = "checked_out"
	ReasonBaseBranch ProtectionReason = "base_branch"
	ReasonWorktree   ProtectionReason = "worktree"
	ReasonTaskInUse  ProtectionReason = "task_in_use"
	ReasonRemoteHead ProtectionReason = "remote_head"
)

// remoteHeadName is the symbolic default ref of a remote (origin/HEAD); it is never deletable.
const remoteHeadName = "HEAD"

// Branch is one local or remote-tracking branch of a workspace repository.
type Branch struct {
	// Name is the local name, or the branch name of a remote-tracking ref without its remote ("develop" for origin/develop).
	Name string
	// Remote is true for remote-tracking branches.
	Remote bool
	// RemoteName is the remote of a remote-tracking branch ("origin"); empty for local branches.
	RemoteName string
	// Current is true when the branch is checked out in the workspace root (HEAD).
	Current bool
	// CheckedOut is true when any worktree of the repository has the branch checked out.
	CheckedOut bool
	// Protected is true when the branch must not be deleted; Reason says why.
	Protected bool
	Reason    ProtectionReason
}

// FullName is the name users see: "develop" for a local branch, "origin/develop" for a remote one.
func (b Branch) FullName() string {
	if !b.Remote {
		return b.Name
	}
	return b.RemoteName + "/" + b.Name
}

// Ref returns the reference that identifies the branch in a request.
func (b Branch) Ref() Ref {
	if !b.Remote {
		return Ref{Kind: KindLocal, Name: b.Name}
	}
	return Ref{Kind: KindRemote, Name: b.FullName()}
}

// Matches reports whether the branch is the one the reference names.
func (b Branch) Matches(ref Ref) bool {
	return b.Ref() == ref
}

// Guard holds the facts that protect branches: the base branch of the workspace and the branch names of the live tasks.
type Guard struct {
	BaseBranch string
	// Remotes are the remote names configured in the repository. A base branch is in remote form only when it
	// starts with one of them followed by a slash ("origin/develop"); the list decides, not the slash alone.
	Remotes      []string
	TaskBranches []string
}

// Protect fills in whether, and why, the workspace state protects the branch. The checks follow S5: the
// checked-out branch, the base branch, branches checked out in any worktree, the branches of live tasks and the
// remote default ref. Task branches are compared by local name; the base branch is compared as described in
// isBaseBranch.
func Protect(branch Branch, guard Guard) Branch {
	branch.Protected = true
	switch {
	case branch.Remote && branch.Name == remoteHeadName:
		branch.Reason = ReasonRemoteHead
	case branch.Current:
		branch.Reason = ReasonCheckedOut
	case isBaseBranch(branch, guard):
		branch.Reason = ReasonBaseBranch
	case branch.CheckedOut:
		branch.Reason = ReasonWorktree
	case contains(guard.TaskBranches, branch.Name):
		branch.Reason = ReasonTaskInUse
	default:
		branch.Protected = false
		branch.Reason = ReasonNone
	}
	return branch
}

// SortBranches orders local branches first, then remote ones, each group by full name.
func SortBranches(branches []Branch) {
	sort.SliceStable(branches, func(i, j int) bool {
		if branches[i].Remote != branches[j].Remote {
			return !branches[i].Remote
		}
		return branches[i].FullName() < branches[j].FullName()
	})
}

// Ref identifies a branch in a delete request. For remote branches Name is the full name ("origin/develop").
type Ref struct {
	Kind Kind
	Name string
}

// ParseRef reads a value of the form "local:name" or "remote:origin/name". A malformed value keeps its raw text
// as the name with an empty kind, so ValidateRef rejects it and the request still reports that entry.
func ParseRef(value string) Ref {
	kind, name, found := strings.Cut(value, ":")
	if found {
		switch Kind(kind) {
		case KindLocal, KindRemote:
			return Ref{Kind: Kind(kind), Name: name}
		}
	}
	return Ref{Name: value}
}

// String is the canonical value of the reference, the one the forms send.
func (r Ref) String() string {
	if r.Kind == "" {
		return r.Name
	}
	return fmt.Sprintf("%s:%s", r.Kind, r.Name)
}

// ValidateRef rejects references git must never see: unknown kinds, empty names, names git would read as an
// option (leading "-"), NUL bytes, and remote references without a branch part.
func ValidateRef(ref Ref) error {
	if ref.Kind != KindLocal && ref.Kind != KindRemote {
		return werrors.ErrInvalidBranch
	}
	if ref.Name == "" || strings.HasPrefix(ref.Name, "-") || strings.ContainsRune(ref.Name, 0) {
		return werrors.ErrInvalidBranch
	}
	if ref.Kind == KindRemote {
		remote, branch, found := strings.Cut(ref.Name, "/")
		if !found || remote == "" || branch == "" || strings.HasPrefix(remote, "-") || strings.HasPrefix(branch, "-") {
			return werrors.ErrInvalidBranch
		}
	}
	return nil
}

// isBaseBranch reports whether branch is the workspace base. A base in remote form ("origin/develop", with origin
// among the configured remotes) protects the local "develop" and every remote branch named "develop"; the remote
// branch "origin/develop" is protected by its full name. A plain base ("develop", or "feature/x" when no listed
// remote matches) protects its own local name and the remote branches of the same name, as before.
func isBaseBranch(branch Branch, guard Guard) bool {
	if guard.BaseBranch == "" {
		return false
	}
	short, remoteForm := splitBase(guard.BaseBranch, guard.Remotes)
	if branch.Remote {
		return branch.FullName() == guard.BaseBranch || branch.Name == short
	}
	return branch.Name == guard.BaseBranch || (remoteForm && branch.Name == short)
}

// splitBase returns the short name of a base in remote form, and whether it is in remote form at all.
func splitBase(base string, remotes []string) (string, bool) {
	if _, name, ok := SplitRemoteRef(base, remotes); ok {
		return name, true
	}
	return base, false
}

// SplitRemoteRef splits a remote-tracking name such as "origin/develop" into its remote ("origin") and branch
// ("develop") using the configured remotes, which may contain slashes. When several remotes match, the longest one
// wins: with remotes "a" and "a/b", "a/b/develop" is branch develop of remote a/b. The boolean is false when no
// remote matches.
func SplitRemoteRef(full string, remotes []string) (string, string, bool) {
	var remote, name string
	found := false
	for _, candidate := range remotes {
		if candidate == "" {
			continue
		}
		rest, ok := strings.CutPrefix(full, candidate+"/")
		if !ok || rest == "" {
			continue
		}
		if !found || len(candidate) > len(remote) {
			remote, name, found = candidate, rest, true
		}
	}
	return remote, name, found
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
