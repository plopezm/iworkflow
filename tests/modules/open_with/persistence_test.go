package open_with_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/outbound/persistence"
)

func launcher(t *testing.T, id string, name string, args []string, env []models.EnvVar) models.Launcher {
	t.Helper()
	built, err := models.NewLauncher(id, name, "/usr/bin/"+name, args, env)
	require.NoError(t, err)
	return built
}

func TestRepositorySavesAndReadsBackArgsAndEnvironment(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewLauncherRepository(dbtest.New(t))
	original := launcher(t, "a1", "code", []string{"--new-window", "a b"}, []models.EnvVar{{Name: "K", Value: "v=1"}})

	require.NoError(t, repo.Save(ctx, original))
	loaded, err := repo.Get(ctx, "a1")

	require.NoError(t, err)
	require.Equal(t, original, loaded)
}

func TestRepositoryListsInCreationOrderAndStoresOptionalFieldsEmpty(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewLauncherRepository(dbtest.New(t))
	require.NoError(t, repo.Save(ctx, launcher(t, "z", "zed", nil, nil)))
	require.NoError(t, repo.Save(ctx, launcher(t, "a", "alacritty", nil, nil)))

	list, err := repo.List(ctx)

	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "z", list[0].ID())
	require.Empty(t, list[0].Args())
	require.Empty(t, list[0].Env())
}

func TestRepositoryReportsUnknownIDs(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewLauncherRepository(dbtest.New(t))

	_, err := repo.Get(ctx, "nope")
	require.ErrorIs(t, err, openerrors.ErrLauncherNotFound)
	require.ErrorIs(t, repo.Delete(ctx, "nope"), openerrors.ErrLauncherNotFound)
}

func TestRepositoryDeleteRemovesTheRow(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewLauncherRepository(dbtest.New(t))
	require.NoError(t, repo.Save(ctx, launcher(t, "a1", "code", nil, nil)))

	require.NoError(t, repo.Delete(ctx, "a1"))
	list, err := repo.List(ctx)

	require.NoError(t, err)
	require.Empty(t, list)
}
