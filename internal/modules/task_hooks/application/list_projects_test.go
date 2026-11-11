package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hooksapp "iworkflow/internal/modules/task_hooks/application"
)

// fakeProjects gives a fixed list of projects, or fails.
type fakeProjects struct {
	projects []hooksapp.ProjectOption
	err      error
}

func (f *fakeProjects) Projects(context.Context) ([]hooksapp.ProjectOption, error) {
	return f.projects, f.err
}

func TestListProjectsReturnsTheProjectsOfTheWorkspaces(t *testing.T) {
	t.Parallel()
	want := []hooksapp.ProjectOption{{ID: "ws-1", Name: "alpha"}, {ID: "ws-2", Name: "beta"}}
	got, err := hooksapp.NewListProjects(&fakeProjects{projects: want}).Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestListProjectsWrapsTheLookupError(t *testing.T) {
	t.Parallel()
	cause := errors.New("workspaces unavailable")
	_, err := hooksapp.NewListProjects(&fakeProjects{err: cause}).Execute(context.Background())
	require.ErrorIs(t, err, cause)
}
