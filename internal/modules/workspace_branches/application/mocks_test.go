package application

import (
	"context"
	"errors"
	"fmt"

	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	"iworkflow/internal/modules/workspace_branches/domain/models"
	"iworkflow/internal/modules/workspace_branches/domain/ports/outbound"
)

// fakeWorkspaces resolves the workspaces of the test; an unknown id wraps ErrWorkspaceNotFound.
type fakeWorkspaces struct {
	byID map[string]outbound.WorkspaceTarget
}

func (f *fakeWorkspaces) Resolve(_ context.Context, id string) (outbound.WorkspaceTarget, error) {
	workspace, ok := f.byID[id]
	if !ok {
		return outbound.WorkspaceTarget{}, fmt.Errorf("workspace %q: %w", id, werrors.ErrWorkspaceNotFound)
	}
	return workspace, nil
}

// fakeTasks gives the branches of the live tasks.
type fakeTasks struct {
	names []string
	err   error
}

func (f *fakeTasks) InUse(context.Context, string) ([]string, error) {
	return f.names, f.err
}

// fakeGit records every call and answers from its fields.
type fakeGit struct {
	branches []models.Branch
	remotes  []string
	listErr  error
	// failLocal and failRemote make the named branch fail to delete.
	failLocal  map[string]error
	failRemote map[string]error
	calls      []string
}

func (f *fakeGit) ListBranches(_ context.Context, repoPath string) ([]models.Branch, error) {
	f.calls = append(f.calls, "list "+repoPath)
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]models.Branch, len(f.branches))
	copy(out, f.branches)
	return out, nil
}

func (f *fakeGit) ListRemotes(_ context.Context, _ string) ([]string, error) {
	return f.remotes, nil
}

func (f *fakeGit) DeleteLocal(_ context.Context, repoPath string, name string) error {
	f.calls = append(f.calls, "delete-local "+repoPath+" "+name)
	if err := f.failLocal[name]; err != nil {
		return err
	}
	return nil
}

func (f *fakeGit) DeleteRemote(_ context.Context, repoPath string, remote string, name string) error {
	f.calls = append(f.calls, "delete-remote "+repoPath+" "+remote+" "+name)
	if err := f.failRemote[name]; err != nil {
		return err
	}
	return nil
}

// deletions returns the calls that changed the repository.
func (f *fakeGit) deletions() []string {
	var out []string
	for _, call := range f.calls {
		if len(call) > 6 && call[:6] == "delete" {
			out = append(out, call)
		}
	}
	return out
}

var errGitRefused = errors.New("git refused")
