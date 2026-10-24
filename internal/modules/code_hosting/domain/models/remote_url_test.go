package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/domain/models"
)

func parsedRemote(t *testing.T, raw string) models.RemoteURL {
	t.Helper()
	remote, err := models.ParseRemoteURL(raw)
	require.NoError(t, err, raw)
	return remote
}

func TestRemoteURLParsesHTTPSWithGitSuffix(t *testing.T) {
	remote := parsedRemote(t, "https://GitLab.Corp.local/grupo/sub/proyecto.git")
	require.Equal(t, "gitlab.corp.local", remote.Host)
	require.Equal(t, "grupo/sub/proyecto", remote.Path)
}

func TestRemoteURLParsesHTTPSWithCredentialsAndPort(t *testing.T) {
	remote := parsedRemote(t, "https://oauth2:abc@git.corp:8443/g/p")
	require.Equal(t, "git.corp", remote.Host)
	require.Equal(t, "g/p", remote.Path)
}

func TestRemoteURLParsesScpStyle(t *testing.T) {
	remote := parsedRemote(t, "git@git.corp:grupo/proyecto.git")
	require.Equal(t, "git.corp", remote.Host)
	require.Equal(t, "grupo/proyecto", remote.Path)
}

func TestRemoteURLParsesSSHSchemeWithPort(t *testing.T) {
	remote := parsedRemote(t, "ssh://git@git.corp:2222/grupo/proyecto.git")
	require.Equal(t, "git.corp", remote.Host)
	require.Equal(t, "grupo/proyecto", remote.Path)
}

func TestRemoteURLRejectsLocalPathsAndUnknownSchemes(t *testing.T) {
	for _, raw := range []string{"/srv/repos/p.git", "../p", "file:///srv/p.git", "https://host", ""} {
		_, err := models.ParseRemoteURL(raw)
		require.Error(t, err, raw)
	}
}

func TestRemoteURLErrorNeverLeaksCredentials(t *testing.T) {
	_, err := models.ParseRemoteURL("https://u:topsecret@host")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "topsecret")
}
