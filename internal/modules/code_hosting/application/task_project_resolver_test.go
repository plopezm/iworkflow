package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

func TestResolveAppliesTheGitLabSettingsForAGitLabOrigin(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = []models.CodeHostSettings{githubSettings(t), gitlabSettings(t)}

	resolved, err := f.resolver().Resolve(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, models.GitLab, resolved.Settings.Provider())
	require.Equal(t, project, resolved.Project)
}

func TestResolveAppliesTheGitHubSettingsForAGitHubOrigin(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = []models.CodeHostSettings{gitlabSettings(t), githubSettings(t)}
	f.remote.origin = "git@github.com:acme/app.git"

	resolved, err := f.resolver().Resolve(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, models.GitHub, resolved.Settings.Provider())
	require.Equal(t, "acme/app", resolved.Project)
}

func TestResolveAGitHubOriginWithoutGitHubSettingsAsksToConfigureIt(t *testing.T) {
	f := newFixture(t)
	f.remote.origin = "https://github.com/acme/app.git"

	_, err := f.resolver().Resolve(context.Background(), taskID)

	require.Equal(t, codeerrors.NotConfigured(models.GitHub), err)
}

func TestResolveAnOriginMatchingNoProviderIsReportedWithItsHost(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = []models.CodeHostSettings{gitlabSettings(t), githubSettings(t)}
	f.remote.origin = "https://otro.io/a/b.git"

	_, err := f.resolver().Resolve(context.Background(), taskID)

	require.Equal(t, codeerrors.UnknownHost("otro.io"), err)
}

func TestResolveUnknownTaskFailsBeforeAnyGitOrProviderCall(t *testing.T) {
	f := newFixture(t)
	f.tasks.source = nil

	_, err := f.resolver().Resolve(context.Background(), taskID)

	require.Equal(t, codeerrors.TaskNotFound(taskID), err)
	require.Empty(t, f.remote.publishes)
}

func TestResolveGivesTheClientOfTheDetectedProvider(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = []models.CodeHostSettings{gitlabSettings(t), githubSettings(t)}
	f.factory.byProvider = map[models.HostingProvider]outbound.CodeHostClient{
		models.GitLab: &fakeClient{user: "gitlab-user"},
		models.GitHub: &fakeClient{user: "github-user"},
	}
	f.remote.origin = "git@github.com:acme/app.git"

	resolved, err := f.resolver().Resolve(context.Background(), taskID)

	require.NoError(t, err)
	user, err := resolved.Client.CurrentUser(context.Background(), resolved.Settings)
	require.NoError(t, err)
	require.Equal(t, "github-user", user)
}

func TestStatusOfReportsTheDetectedProvider(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = []models.CodeHostSettings{githubSettings(t)}
	f.remote.origin = "git@github.com:acme/app.git"
	resolved, err := f.resolver().Resolve(context.Background(), taskID)
	require.NoError(t, err)

	status, err := f.resolver().StatusOf(context.Background(), resolved, mergeRequest(3, "opened"))

	require.NoError(t, err)
	require.Equal(t, models.GitHub, status.Provider)
	require.Equal(t, "acme/app", status.ProjectPath)
	require.Nil(t, status.Pipeline)
}
