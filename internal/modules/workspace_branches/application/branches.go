// Package application holds the use cases of the workspace_branches module: listing the branches of a
// workspace and deleting a batch of them. Protection (S5) is decided here, from fresh state on every call.
package application

import (
	"context"
	"fmt"
	"strings"

	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	"iworkflow/internal/modules/workspace_branches/domain/models"
	"iworkflow/internal/modules/workspace_branches/domain/ports/outbound"
)

// branchSource loads the branches of a workspace with their protection already decided. It is the one place
// that combines the git view with the workspace base branch and the live task branches.
type branchSource struct {
	workspaces outbound.WorkspaceResolver
	git        outbound.BranchGit
	tasks      outbound.ProtectedBranches
}

// load resolves the workspace and returns its branches, sorted and protected.
func (s branchSource) load(ctx context.Context, workspaceID string) (outbound.WorkspaceTarget, []models.Branch, error) {
	workspace, err := s.workspaces.Resolve(ctx, workspaceID)
	if err != nil {
		return outbound.WorkspaceTarget{}, nil, fmt.Errorf("resolve workspace: %w", err)
	}
	remotes, err := s.git.ListRemotes(ctx, workspace.Path)
	if err != nil {
		return outbound.WorkspaceTarget{}, nil, fmt.Errorf("list remotes: %w", err)
	}
	raw, err := s.git.ListBranches(ctx, workspace.Path)
	if err != nil {
		return outbound.WorkspaceTarget{}, nil, fmt.Errorf("list branches: %w", err)
	}
	inUse, err := s.tasks.InUse(ctx, workspaceID)
	if err != nil {
		return outbound.WorkspaceTarget{}, nil, fmt.Errorf("list task branches: %w", err)
	}
	// The base is resolved against the real remotes, so a base in remote form is recognised.
	guard := models.Guard{BaseBranch: workspace.BaseBranch, Remotes: remotes, TaskBranches: inUse}
	branches := make([]models.Branch, 0, len(raw))
	for _, branch := range raw {
		branches = append(branches, models.Protect(branch, guard))
	}
	models.SortBranches(branches)
	return workspace, branches, nil
}

// ListBranches returns the branches of a workspace, optionally narrowed by a case-insensitive filter on their name.
type ListBranches struct {
	source branchSource
}

// NewListBranches builds the use case over the ports.
func NewListBranches(workspaces outbound.WorkspaceResolver, git outbound.BranchGit, tasks outbound.ProtectedBranches) *ListBranches {
	return &ListBranches{source: branchSource{workspaces: workspaces, git: git, tasks: tasks}}
}

// BranchList is the answer of ListBranches: the workspace and its matching branches.
type BranchList struct {
	Workspace outbound.WorkspaceTarget
	Branches  []models.Branch
	// Filter is the filter that was applied, trimmed.
	Filter string
}

// Execute lists the branches whose full name contains filter (every branch when filter is blank).
func (u *ListBranches) Execute(ctx context.Context, workspaceID string, filter string) (BranchList, error) {
	workspace, branches, err := u.source.load(ctx, workspaceID)
	if err != nil {
		return BranchList{}, err
	}
	needle := strings.ToLower(strings.TrimSpace(filter))
	matching := make([]models.Branch, 0, len(branches))
	for _, branch := range branches {
		if needle == "" || strings.Contains(strings.ToLower(branch.FullName()), needle) {
			matching = append(matching, branch)
		}
	}
	return BranchList{Workspace: workspace, Branches: matching, Filter: strings.TrimSpace(filter)}, nil
}

// DeleteOutcome is what happened to one reference of a delete request.
type DeleteOutcome string

// Outcomes of a delete request entry.
const (
	OutcomeDeleted DeleteOutcome = "deleted"
	OutcomeRefused DeleteOutcome = "refused"
	OutcomeFailed  DeleteOutcome = "failed"
)

// DeleteResult reports one reference of a batch. A refused or failed entry never stops the others.
type DeleteResult struct {
	Ref     models.Ref
	Outcome DeleteOutcome
	// Reason is set when the branch is protected (Outcome refused).
	Reason models.ProtectionReason
	// Err is set when the entry failed: a domain sentinel (invalid, not found) or the git failure.
	Err error
}

// DeleteBranches deletes a batch of branches of one workspace, one result per reference.
type DeleteBranches struct {
	source branchSource
	git    outbound.BranchGit
}

// NewDeleteBranches builds the use case over the ports.
func NewDeleteBranches(workspaces outbound.WorkspaceResolver, git outbound.BranchGit, tasks outbound.ProtectedBranches) *DeleteBranches {
	source := branchSource{workspaces: workspaces, git: git, tasks: tasks}
	return &DeleteBranches{source: source, git: git}
}

// Execute deletes the referenced branches. The branch list is read once, before the first deletion, so every
// reference is checked against the same fresh state. A reference is deleted only when it names a listed,
// unprotected branch; a remote one only when it exists as a remote-tracking ref. Each failure is recorded and
// the rest of the batch still runs. Only a failure to resolve the workspace or list the branches is returned.
func (u *DeleteBranches) Execute(ctx context.Context, workspaceID string, refs []models.Ref) ([]DeleteResult, error) {
	workspace, branches, err := u.source.load(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	results := make([]DeleteResult, 0, len(refs))
	for _, ref := range refs {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return results, fmt.Errorf("delete branches: %w", ctxErr)
		}
		results = append(results, u.deleteOne(ctx, workspace.Path, branches, ref))
	}
	return results, nil
}

func (u *DeleteBranches) deleteOne(ctx context.Context, repoPath string, branches []models.Branch, ref models.Ref) DeleteResult {
	if err := models.ValidateRef(ref); err != nil {
		return DeleteResult{Ref: ref, Outcome: OutcomeFailed, Err: err}
	}
	branch, found := findBranch(branches, ref)
	if !found {
		return DeleteResult{Ref: ref, Outcome: OutcomeFailed, Err: werrors.ErrBranchNotFound}
	}
	if branch.Protected {
		return DeleteResult{Ref: ref, Outcome: OutcomeRefused, Reason: branch.Reason}
	}
	var err error
	if branch.Remote {
		err = u.git.DeleteRemote(ctx, repoPath, branch.RemoteName, branch.Name)
	} else {
		err = u.git.DeleteLocal(ctx, repoPath, branch.Name)
	}
	if err != nil {
		return DeleteResult{Ref: ref, Outcome: OutcomeFailed, Err: err}
	}
	return DeleteResult{Ref: ref, Outcome: OutcomeDeleted}
}

func findBranch(branches []models.Branch, ref models.Ref) (models.Branch, bool) {
	for _, branch := range branches {
		if branch.Matches(ref) {
			return branch, true
		}
	}
	return models.Branch{}, false
}
