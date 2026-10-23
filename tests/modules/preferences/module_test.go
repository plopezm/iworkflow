package preferences_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/infrastructure/di"
)

func TestModuleWiresEveryUseCaseOnOneDatabase(t *testing.T) {
	ctx := context.Background()
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})

	saved, err := module.SaveLanguage.Execute(ctx, "es")
	require.NoError(t, err)
	require.Equal(t, "es", saved.Code())
	language, err := module.GetLanguage.Execute(ctx)
	require.NoError(t, err)
	require.Equal(t, "es", language.Code())

	enabled, err := module.SaveNetworkAccess.Execute(ctx, true)
	require.NoError(t, err)
	require.True(t, enabled)
	networkAccess, err := module.GetNetworkAccess.Execute(ctx)
	require.NoError(t, err)
	require.True(t, networkAccess)

	port, err := module.SaveListenPort.Execute(ctx, 9100)
	require.NoError(t, err)
	require.Equal(t, uint16(9100), port.Port())
	storedPort, err := module.GetListenPort.Execute(ctx)
	require.NoError(t, err)
	require.Equal(t, uint16(9100), storedPort.Port())

	required, err := module.SaveRequireAccessToken.Execute(ctx, false)
	require.NoError(t, err)
	require.False(t, required)
	requireToken, err := module.GetRequireAccessToken.Execute(ctx)
	require.NoError(t, err)
	require.False(t, requireToken)
}

func TestAccessTokenIsGeneratedOnceThenStableUntilRegenerated(t *testing.T) {
	ctx := context.Background()
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})

	first, err := module.GetAccessToken.Execute(ctx)
	require.NoError(t, err)
	again, err := module.GetAccessToken.Execute(ctx)
	require.NoError(t, err)
	require.Equal(t, first, again)

	fresh, err := module.RegenerateAccessToken.Execute(ctx)
	require.NoError(t, err)
	require.NotEqual(t, first, fresh)
	current, err := module.GetAccessToken.Execute(ctx)
	require.NoError(t, err)
	require.Equal(t, fresh, current)
}

func TestPrivilegedPortIsRejectedAndNeverStored(t *testing.T) {
	ctx := context.Background()
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})

	_, err := module.SaveListenPort.Execute(ctx, 80)

	var invalid *domainerrors.InvalidPortError
	require.True(t, errors.As(err, &invalid))
	stored, err := module.GetListenPort.Execute(ctx)
	require.NoError(t, err)
	require.Nil(t, stored)
}
