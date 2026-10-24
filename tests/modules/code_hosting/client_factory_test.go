package code_hosting_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/api"
)

func TestClientManagerReusesOneClientPerTLSMode(t *testing.T) {
	manager := api.NewClientManager()

	strict := manager.Client(false)
	require.Same(t, strict, manager.Client(false))
	insecure := manager.Client(true)
	require.Same(t, insecure, manager.Client(true))
	require.NotSame(t, strict, insecure)
}

func TestClientManagerOnlyDisablesVerificationForTheInsecureClient(t *testing.T) {
	manager := api.NewClientManager()

	strictTransport, ok := manager.Client(false).Transport.(*http.Transport)
	require.True(t, ok)
	insecureTransport, ok := manager.Client(true).Transport.(*http.Transport)
	require.True(t, ok)

	require.False(t, strictTransport.TLSClientConfig.InsecureSkipVerify)
	require.True(t, insecureTransport.TLSClientConfig.InsecureSkipVerify)
	require.Equal(t, uint16(tls.VersionTLS12), strictTransport.TLSClientConfig.MinVersion)
}

func TestClientManagerNeverFollowsRedirects(t *testing.T) {
	client := api.NewClientManager().Client(false)

	require.Error(t, client.CheckRedirect(nil, nil), "a redirect must be returned, not followed")
}

func TestClientFactoryGivesGitLabItsClient(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{target: "GET /api/v4/user", status: 200, body: `{"username":"desde-gitlab"}`},
		scriptedRoute{target: "GET /api/v3/user", status: 200, body: `{"login":"desde-github"}`},
	)
	factory := api.NewClientFactory(api.NewClientManager())
	settings := settingsFor(t, models.GitLab, provider.URL(), "tok")

	user, err := factory.ClientFor(models.GitLab).CurrentUser(context.Background(), settings)

	require.NoError(t, err)
	require.Equal(t, "desde-gitlab", user)
}

func TestClientFactoryGivesGitHubItsClient(t *testing.T) {
	provider := newFakeProvider(t,
		scriptedRoute{target: "GET /api/v4/user", status: 200, body: `{"username":"desde-gitlab"}`},
		scriptedRoute{target: "GET /api/v3/user", status: 200, body: `{"login":"desde-github"}`},
	)
	factory := api.NewClientFactory(api.NewClientManager())
	settings := settingsFor(t, models.GitHub, provider.URL(), "tok")

	user, err := factory.ClientFor(models.GitHub).CurrentUser(context.Background(), settings)

	require.NoError(t, err)
	require.Equal(t, "desde-github", user)
}
