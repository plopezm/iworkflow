package application_test

import (
	"context"

	"github.com/google/uuid"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
)

// fakeWorkspaceRoots resolves every scope to root, or fails with err. It records the scopes asked.
type fakeWorkspaceRoots struct {
	root  models.WorkspaceRoot
	err   error
	calls []models.ProjectScope
}

func (f *fakeWorkspaceRoots) Resolve(_ context.Context, scope models.ProjectScope) (models.WorkspaceRoot, error) {
	f.calls = append(f.calls, scope)
	if f.err != nil {
		return models.WorkspaceRoot{}, f.err
	}
	return f.root, nil
}

// fakeDirectoryReader records the roots and relatives it is asked to list.
type fakeDirectoryReader struct {
	contents models.DirectoryContents
	err      error
	roots    []string
	relative []string
}

func (f *fakeDirectoryReader) List(_ context.Context, root string, relative string) (models.DirectoryContents, error) {
	f.roots = append(f.roots, root)
	f.relative = append(f.relative, relative)
	if f.err != nil {
		return models.DirectoryContents{}, f.err
	}
	return f.contents, nil
}

// fileWrite is one call received by fakeFileStore.Write.
type fileWrite struct {
	root     string
	relative string
	content  string
}

// fakeFileStore returns file on Read and records every Write.
type fakeFileStore struct {
	file   models.ProjectFile
	err    error
	reads  int
	writes []fileWrite
}

func (f *fakeFileStore) Read(_ context.Context, _ string, _ string) (models.ProjectFile, error) {
	f.reads++
	if f.err != nil {
		return models.ProjectFile{}, f.err
	}
	return f.file, nil
}

func (f *fakeFileStore) Write(_ context.Context, root string, relative string, content string) error {
	f.writes = append(f.writes, fileWrite{root: root, relative: relative, content: content})
	return f.err
}

// fakeFileSearch returns a fixed listing and records the roots and the calls.
type fakeFileSearch struct {
	files []string
	err   error
	roots []string
}

func (f *fakeFileSearch) Files(_ context.Context, root string) ([]string, error) {
	f.roots = append(f.roots, root)
	if f.err != nil {
		return nil, f.err
	}
	return f.files, nil
}

// fakeGitStatus records every git call. Set the *Err fields to inject failures.
type fakeGitStatus struct {
	changes    []models.GitChange
	changesErr error
	base       *string
	baseErr    error
	stageErr   error
	unstageErr error
	commitErr  error

	// calls counts every port call, so tests can assert that git was never reached.
	calls int

	changesRoot string
	baseRoot    string
	baseRel     string
	stageRoot   string
	staged      []string
	unstageRoot string
	unstaged    []string
	commitRoot  string
	commitMsg   string
}

func (f *fakeGitStatus) Changes(_ context.Context, root string) ([]models.GitChange, error) {
	f.calls++
	f.changesRoot = root
	if f.changesErr != nil {
		return nil, f.changesErr
	}
	return f.changes, nil
}

func (f *fakeGitStatus) BaseContent(_ context.Context, root string, relative string) (*string, error) {
	f.calls++
	f.baseRoot = root
	f.baseRel = relative
	if f.baseErr != nil {
		return nil, f.baseErr
	}
	return f.base, nil
}

func (f *fakeGitStatus) Stage(_ context.Context, root string, paths []string) error {
	f.calls++
	f.stageRoot = root
	f.staged = paths
	return f.stageErr
}

func (f *fakeGitStatus) Unstage(_ context.Context, root string, paths []string) error {
	f.calls++
	f.unstageRoot = root
	f.unstaged = paths
	return f.unstageErr
}

func (f *fakeGitStatus) Commit(_ context.Context, root string, message string) error {
	f.calls++
	f.commitRoot = root
	f.commitMsg = message
	return f.commitErr
}

// workspaceRoot is the root every fake workspace resolves to in these tests.
func workspaceRoot() models.WorkspaceRoot {
	return models.WorkspaceRoot{Name: "demo", Path: "/tmp/demo"}
}

// anyScope is a scope whose identifier does not matter to the test.
func anyScope() models.ProjectScope {
	return models.WorkspaceScope(uuid.New())
}

// unknownScopeErr is the error a resolver returns for a scope it does not know.
func unknownScopeErr() error {
	return treeerrors.WorkspaceNotFound()
}
