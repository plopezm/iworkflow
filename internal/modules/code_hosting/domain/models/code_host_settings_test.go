package models_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
)

func gitlabSettings(t *testing.T, baseURL string) models.CodeHostSettings {
	t.Helper()
	settings, err := models.NewCodeHostSettings(models.GitLab, baseURL, "tok", false)
	require.NoError(t, err)
	return settings
}

func githubSettings(t *testing.T, baseURL string) models.CodeHostSettings {
	t.Helper()
	settings, err := models.NewCodeHostSettings(models.GitHub, baseURL, "tok", false)
	require.NoError(t, err)
	return settings
}

func TestPushURLJoinsBaseAndProjectWithoutCredentials(t *testing.T) {
	require.Equal(t, "https://git.corp/gitlab/grupo/sub/proyecto.git",
		gitlabSettings(t, "https://git.corp/gitlab/").PushURL("/grupo/sub/proyecto/"))
	require.Equal(t, "https://github.com/acme/app.git",
		githubSettings(t, "https://github.com").PushURL("acme/app"))
}

func TestGitUsernameDependsOnTheProvider(t *testing.T) {
	require.Equal(t, "oauth2", gitlabSettings(t, "https://git.corp").GitUsername())
	require.Equal(t, "x-access-token", githubSettings(t, "https://github.com").GitUsername())
}

func TestNormalizesURLAndBuildsAPIURL(t *testing.T) {
	settings := gitlabSettings(t, "  https://git.corp/ ")
	require.Equal(t, "https://git.corp", settings.BaseURL())
	require.Equal(t, "https://git.corp/api/v4", settings.APIURL())
}

func TestRejectsInvalidURLAndEmptyToken(t *testing.T) {
	for _, rawURL := range []string{"git.corp", "ftp://git.corp", "https://", "https://u:p@git.corp"} {
		_, err := models.NewCodeHostSettings(models.GitLab, rawURL, "t", false)
		require.Error(t, err, rawURL)
	}
	_, err := models.NewCodeHostSettings(models.GitLab, "https://git.corp", "  ", false)
	require.ErrorIs(t, err, codeerrors.MissingToken())
	require.Equal(t, "el token es obligatorio", err.Error())
}

func TestStringAndGoStringHideToken(t *testing.T) {
	settings, err := models.NewCodeHostSettings(models.GitLab, "https://git.corp", "abcdef123", false)
	require.NoError(t, err)
	require.NotContains(t, fmt.Sprintf("%v %+v %#v", settings, settings, settings), "abcdef123")
}

func TestProjectPathMatchesHostIgnoringPortAndScheme(t *testing.T) {
	path, err := gitlabSettings(t, "https://git.corp:8443").ProjectPath(parsedRemote(t, "git@GIT.corp:g/p.git"))
	require.NoError(t, err)
	require.Equal(t, "g/p", path)
}

func TestProjectPathRejectsOtherHost(t *testing.T) {
	_, err := gitlabSettings(t, "https://git.corp").ProjectPath(parsedRemote(t, "https://github.com/a/b"))
	require.ErrorIs(t, err, codeerrors.HostMismatch("", ""))
	require.Equal(t, "el remoto origin (github.com) no pertenece a la instancia configurada (git.corp)", err.Error())
}

func TestProjectPathStripsRelativeURLRoot(t *testing.T) {
	path, err := gitlabSettings(t, "https://corp.local/gitlab").ProjectPath(parsedRemote(t, "https://corp.local/gitlab/g/p.git"))
	require.NoError(t, err)
	require.Equal(t, "g/p", path)
}

func TestGitHubComUsesThePublicAPIHost(t *testing.T) {
	require.Equal(t, "https://api.github.com", githubSettings(t, "https://github.com").APIURL())
}

func TestGitHubEnterpriseUsesAPIv3UnderTheBaseURL(t *testing.T) {
	require.Equal(t, "https://ghe.corp/api/v3", githubSettings(t, "https://ghe.corp/").APIURL())
}

func TestGitHubProjectPathIsOwnerAndRepo(t *testing.T) {
	settings := githubSettings(t, "https://github.com")
	for _, raw := range []string{
		"git@github.com:acme/app.git",
		"https://github.com/acme/app",
		"ssh://git@ssh.github.com:443/acme/app.git",
	} {
		path, err := settings.ProjectPath(parsedRemote(t, raw))
		require.NoError(t, err, raw)
		require.Equal(t, "acme/app", path, raw)
	}
}

func TestGitHubProjectPathRejectsNestedGroups(t *testing.T) {
	_, err := githubSettings(t, "https://github.com").ProjectPath(parsedRemote(t, "https://github.com/a/b/c"))
	require.ErrorIs(t, err, codeerrors.InvalidProjectPath(models.GitHub, ""))
}

func TestSSHAliasOnlyMatchesGitHubCom(t *testing.T) {
	require.True(t, githubSettings(t, "https://github.com").MatchesHost("ssh.github.com"))
	require.False(t, githubSettings(t, "https://ghe.corp").MatchesHost("ssh.github.com"))
	require.False(t, gitlabSettings(t, "https://github.com").MatchesHost("ssh.github.com"))
}

func TestGitLabProjectPathKeepsNestedGroups(t *testing.T) {
	path, err := gitlabSettings(t, "https://git.corp").ProjectPath(parsedRemote(t, "https://git.corp/a/b/c"))
	require.NoError(t, err)
	require.Equal(t, "a/b/c", path)
}

func TestSelectForHostPicksTheProviderByOriginHost(t *testing.T) {
	configured := []models.CodeHostSettings{gitlabSettings(t, "https://git.corp"), githubSettings(t, "https://github.com")}
	gitlab, err := models.SelectForHost(configured, "git.corp")
	require.NoError(t, err)
	hub, err := models.SelectForHost(configured, "github.com")
	require.NoError(t, err)
	require.Equal(t, models.GitLab, gitlab.Provider())
	require.Equal(t, models.GitHub, hub.Provider())
}

func TestSelectForHostReportsUnconfiguredGitHubAndUnknownHosts(t *testing.T) {
	configured := []models.CodeHostSettings{gitlabSettings(t, "https://git.corp")}
	_, err := models.SelectForHost(configured, "github.com")
	require.Equal(t, codeerrors.NotConfigured(models.GitHub), err)
	_, err = models.SelectForHost(configured, "other.io")
	require.Equal(t, codeerrors.UnknownHost("other.io"), err)
}
