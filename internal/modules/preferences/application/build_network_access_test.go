package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
	"iworkflow/internal/modules/preferences/domain/models"
)

// newBuilder wires the use case on the fakes; the token generator is the real one.
func newBuilder(repository *fakeRepository, network *fakeNetwork, runtime *fakeRuntime) *application.BuildNetworkAccess {
	tokens := application.NewGetAccessToken(repository, models.GenerateAccessToken)
	return application.NewBuildNetworkAccess(repository, tokens, network, runtime)
}

func TestBuildNetworkAccessDefaultsToPrivateWithNoToken(t *testing.T) {
	repository := newFakeRepository()
	runtime := &fakeRuntime{status: models.ServerStatus{Port: 3000}}

	status, err := newBuilder(repository, &fakeNetwork{}, runtime).Execute(context.Background())

	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.False(t, status.Active)
	require.True(t, status.RequireToken)
	require.Nil(t, status.Token)
	require.Equal(t, uint16(3000), status.ConfiguredPort)
	require.Equal(t, 0, repository.writes, "nothing is generated while the token is not in play")
}

func TestBuildNetworkAccessGeneratesTheTokenWhenItIsInPlay(t *testing.T) {
	repository := newFakeRepository()
	repository.networkAccess = true
	runtime := &fakeRuntime{status: models.ServerStatus{Port: 3000}}

	status, err := newBuilder(repository, &fakeNetwork{}, runtime).Execute(context.Background())

	require.NoError(t, err)
	require.NotNil(t, status.Token)
	require.NotNil(t, repository.accessToken, "the generated token is persisted")
	require.Equal(t, *repository.accessToken, *status.Token)
}

func TestBuildNetworkAccessHidesTheTokenWhenNotRequired(t *testing.T) {
	repository := newFakeRepository()
	repository.networkAccess = true
	repository.requireToken = false
	runtime := &fakeRuntime{status: models.ServerStatus{Port: 3000}}

	status, err := newBuilder(repository, &fakeNetwork{}, runtime).Execute(context.Background())

	require.NoError(t, err)
	require.Nil(t, status.Token)
}

func TestBuildNetworkAccessKeepsTheTokenWhileTheRunningServerRequiresIt(t *testing.T) {
	// Saved choice switched off, but the running server still demands it until the restart.
	repository := newFakeRepository()
	repository.networkAccess = true
	repository.requireToken = false
	runtime := &fakeRuntime{status: models.ServerStatus{Port: 3000, Exposed: true, TokenRequired: true}}

	status, err := newBuilder(repository, &fakeNetwork{}, runtime).Execute(context.Background())

	require.NoError(t, err)
	require.NotNil(t, status.Token)
	require.True(t, status.TokenActive)
}

func TestBuildNetworkAccessUsesTheSavedPortAsTheConfiguredOne(t *testing.T) {
	repository := newFakeRepository()
	saved, err := models.NewListenPort(9100)
	require.NoError(t, err)
	repository.listenPort = &saved
	runtime := &fakeRuntime{status: models.ServerStatus{Port: 3000}}

	status, err := newBuilder(repository, &fakeNetwork{}, runtime).Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, uint16(3000), status.Port, "the running port is what the server listens on now")
	require.Equal(t, uint16(9100), status.ConfiguredPort)
}

func TestBuildNetworkAccessPutsThePreferredInterfaceFirst(t *testing.T) {
	network := &fakeNetwork{
		interfaces: []models.NetworkInterface{
			{Name: "eth0", Address: "192.168.1.5"},
			{Name: "tailscale0", Address: "100.64.0.7"},
		},
		preferred: "100.64.0.7",
		hasPrefer: true,
	}

	status, err := newBuilder(newFakeRepository(), network, &fakeRuntime{}).Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, "tailscale0", status.Targets[0].Name)
	require.Equal(t, "eth0", status.Targets[1].Name)
}

func TestBuildNetworkAccessKeepsTheDetectedOrderWithoutAPreferredInterface(t *testing.T) {
	network := &fakeNetwork{interfaces: []models.NetworkInterface{
		{Name: "eth0", Address: "192.168.1.5"},
		{Name: "tailscale0", Address: "100.64.0.7"},
	}}

	status, err := newBuilder(newFakeRepository(), network, &fakeRuntime{}).Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, "eth0", status.Targets[0].Name)
}

func TestBuildNetworkAccessPropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.readErr = errStorage

	_, err := newBuilder(repository, &fakeNetwork{}, &fakeRuntime{}).Execute(context.Background())

	require.ErrorIs(t, err, errStorage)
}
