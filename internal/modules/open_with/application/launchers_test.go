package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/open_with/application"
	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
)

func stored(t *testing.T) *fakeRepository {
	t.Helper()
	launcher, err := models.NewLauncher("a1", "Editor", "/usr/bin/code", []string{"--new-window"}, []models.EnvVar{{Name: "K", Value: "v"}})
	require.NoError(t, err)
	return &fakeRepository{items: []models.Launcher{launcher}}
}

func TestCreateLauncherStoresAValidLauncherWithOptionalFields(t *testing.T) {
	repo := &fakeRepository{}
	useCase := application.NewCreateLauncher(repo, fixedIDs{})

	created, err := useCase.Execute(context.Background(), application.CreateLauncherInput{Name: " Editor ", Path: "code"})

	require.NoError(t, err)
	require.Equal(t, "id-1", created.ID())
	require.Equal(t, "Editor", created.Name())
	require.Empty(t, created.Args())
	require.Empty(t, created.Env())
	require.Len(t, repo.items, 1)
}

func TestCreateLauncherRejectsInvalidInputWithoutStoringIt(t *testing.T) {
	tests := map[string]application.CreateLauncherInput{
		"blank name":   {Name: " ", Path: "code"},
		"blank path":   {Name: "Editor", Path: ""},
		"bad env name": {Name: "Editor", Path: "code", Env: []models.EnvVar{{Name: "A=B", Value: "x"}}},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepository{}

			_, err := application.NewCreateLauncher(repo, fixedIDs{}).Execute(context.Background(), input)

			require.ErrorIs(t, err, openerrors.ErrInvalidLauncher)
			require.Empty(t, repo.items)
		})
	}
}

func TestLaunchStartsTheLauncherInTheWorktree(t *testing.T) {
	starter := &fakeStarter{}
	useCase := application.NewLaunch(stored(t), fakeWorktrees{path: "/work/tree"}, starter)

	_, err := useCase.Execute(context.Background(), application.LaunchInput{TaskID: "t", LauncherID: "a1"})

	require.NoError(t, err)
	require.Equal(t, []models.LaunchSpec{{
		Path: "/usr/bin/code", Args: []string{"--new-window"}, Env: []models.EnvVar{{Name: "K", Value: "v"}}, Dir: "/work/tree",
	}}, starter.specs)
}

func TestLaunchStartsNothingWhenLauncherOrWorktreeIsUnknown(t *testing.T) {
	tests := map[string]struct {
		launcherID string
		worktrees  fakeWorktrees
		want       error
	}{
		"unknown launcher": {launcherID: "zz", worktrees: fakeWorktrees{path: "/w"}, want: openerrors.ErrLauncherNotFound},
		"no worktree":      {launcherID: "a1", worktrees: fakeWorktrees{err: openerrors.ErrWorktreeNotFound}, want: openerrors.ErrWorktreeNotFound},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			starter := &fakeStarter{}

			_, err := application.NewLaunch(stored(t), tt.worktrees, starter).Execute(context.Background(), application.LaunchInput{TaskID: "t", LauncherID: tt.launcherID})

			require.ErrorIs(t, err, tt.want)
			require.Empty(t, starter.specs)
		})
	}
}

func TestLaunchPropagatesTheStartFailure(t *testing.T) {
	starter := &fakeStarter{err: openerrors.ErrLaunchFailed}

	_, err := application.NewLaunch(stored(t), fakeWorktrees{path: "/w"}, starter).Execute(context.Background(), application.LaunchInput{TaskID: "t", LauncherID: "a1"})

	require.ErrorIs(t, err, openerrors.ErrLaunchFailed)
}

func TestDeleteLauncherRemovesItAndReportsUnknownIDs(t *testing.T) {
	repo := stored(t)
	useCase := application.NewDeleteLauncher(repo)

	require.NoError(t, useCase.Execute(context.Background(), "a1"))
	require.Empty(t, repo.items)
	require.ErrorIs(t, useCase.Execute(context.Background(), "a1"), openerrors.ErrLauncherNotFound)
}

func TestListLaunchersReturnsTheStoredOnes(t *testing.T) {
	list, err := application.NewListLaunchers(stored(t)).Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, list, 1)
}
