package system_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/listen"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/outbound/system"
)

func TestRuntimeReportsAPrivateServer(t *testing.T) {
	status := system.NewServerRuntime(listen.Info{
		Addr: netip.MustParseAddrPort("127.0.0.1:3000"),
	}).Status()

	require.Equal(t, uint16(3000), status.Port)
	require.False(t, status.Exposed)
	require.False(t, status.LockedByEnv)
	require.False(t, status.TokenRequired)
}

func TestRuntimeReportsAnExposedServerWithItsTokenAndEnvLock(t *testing.T) {
	status := system.NewServerRuntime(listen.Info{
		Addr:          netip.MustParseAddrPort("0.0.0.0:9100"),
		LockedByEnv:   true,
		TokenRequired: true,
	}).Status()

	require.Equal(t, uint16(9100), status.Port)
	require.True(t, status.Exposed)
	require.True(t, status.LockedByEnv)
	require.True(t, status.TokenRequired)
}
