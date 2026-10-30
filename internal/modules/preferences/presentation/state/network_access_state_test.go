package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/preferences/presentation/state"
)

// access builds card data with the token state in agreement, so each case changes one thing.
func access(enabled bool, active bool, lockedByEnv bool) dto.NetworkAccessDTO {
	return dto.NewNetworkAccessDTO(enabled, active, lockedByEnv, 3000).WithTokenState(true, true)
}

func TestRestartIsPendingWhenTheChoiceDiffersFromTheRunningServer(t *testing.T) {
	require.True(t, state.NewNetworkAccessView(access(true, false, false)).RestartPending())
	require.True(t, state.NewNetworkAccessView(access(false, true, false)).RestartPending())
}

func TestNoRestartWhenTheyMatchOrTheEnvironmentDecides(t *testing.T) {
	require.False(t, state.NewNetworkAccessView(access(true, true, false)).RestartPending())
	require.False(t, state.NewNetworkAccessView(access(false, false, false)).RestartPending())
	require.False(t, state.NewNetworkAccessView(access(true, false, true)).RestartPending())
}

func TestRestartIsPendingWhenOnlyTheTokenChoiceDiffers(t *testing.T) {
	withoutToken := access(true, true, false).WithTokenState(false, true)
	require.True(t, state.NewNetworkAccessView(withoutToken).RestartPending())

	tokenWasOff := access(true, true, false).WithTokenState(true, false)
	require.True(t, state.NewNetworkAccessView(tokenWasOff).RestartPending())
}

func TestTokenChoiceIsIgnoredWhileTheServerIsPrivate(t *testing.T) {
	private := access(false, false, false).WithTokenState(true, false)

	require.False(t, state.NewNetworkAccessView(private).RestartPending())
}

func TestRestartIsPendingWhenOnlyThePortDiffers(t *testing.T) {
	changed := access(true, true, false).WithConfiguredPort(9100)
	require.True(t, state.NewNetworkAccessView(changed).RestartPending())

	pinned := access(true, true, true).WithConfiguredPort(9100)
	require.False(t, state.NewNetworkAccessView(pinned).RestartPending())
}

func TestPlainURLHintOnlyWhenReachableWithoutInterfaces(t *testing.T) {
	reachable := state.NewNetworkAccessView(access(true, false, false))
	require.Equal(t, "http://<IP>:3000", reachable.PlainURL())

	private := state.NewNetworkAccessView(access(false, false, false))
	require.Empty(t, private.PlainURL())

	withTargets := access(true, false, false).WithTargets([]dto.AccessTargetDTO{{Interface: "eth0", Address: "10.0.0.2"}})
	require.Empty(t, state.NewNetworkAccessView(withTargets).PlainURL())
}

func TestSelectedTargetFallsBackToThePreferredInterface(t *testing.T) {
	view := state.NewNetworkAccessView(access(true, true, false).WithTargets([]dto.AccessTargetDTO{
		{Interface: "eth0", Address: "192.168.1.5"},
		{Interface: "tailscale0", Address: "100.64.0.7"},
	}))

	chosen, ok := view.SelectedTarget("100.64.0.7")
	require.True(t, ok)
	require.Equal(t, "tailscale0", chosen.Interface)

	fallback, ok := view.SelectedTarget("10.9.9.9")
	require.True(t, ok)
	require.Equal(t, "eth0", fallback.Interface)
}

func TestSelectedTargetIsAbsentWithoutInterfaces(t *testing.T) {
	_, ok := state.NewNetworkAccessView(access(false, false, false)).SelectedTarget("10.0.0.2")

	require.False(t, ok)
}

func TestErrorAndRestartingAreAddedWithoutChangingTheData(t *testing.T) {
	base := state.NewNetworkAccessView(access(true, true, false))

	failed := base.WithError("boom")
	restarting := base.WithRestarting()

	require.Equal(t, "boom", failed.Error)
	require.Empty(t, base.Error)
	require.True(t, restarting.Restarting)
	require.False(t, base.Restarting)
}

func TestPortTextIsDecimal(t *testing.T) {
	view := state.NewNetworkAccessView(dto.NewNetworkAccessDTO(true, true, false, 65535))

	require.Equal(t, "http://<IP>:65535", state.NewNetworkAccessView(view.Access.WithTargets(nil)).PlainURL())
}
