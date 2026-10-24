package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/application"
	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

func TestGetCodeHostSettingsReturnsStoredSettings(t *testing.T) {
	f := newFixture(t)

	settings, err := application.NewGetCodeHostSettings(f.settings).Execute(context.Background(), models.GitLab)

	require.NoError(t, err)
	require.NotNil(t, settings)
	require.Equal(t, "https://git.corp", settings.BaseURL())
}

func TestGetCodeHostSettingsReturnsNilWhenUnconfigured(t *testing.T) {
	f := newFixture(t)

	settings, err := application.NewGetCodeHostSettings(f.settings).Execute(context.Background(), models.GitHub)

	require.NoError(t, err)
	require.Nil(t, settings)
}

func TestTestConnectionReturnsTheAuthenticatedUser(t *testing.T) {
	f := newFixture(t)

	user, err := application.NewTestCodeHostConnection(f.settings, f.factory).Execute(context.Background(), models.GitLab)

	require.NoError(t, err)
	require.Equal(t, "pablo", user)
}

func TestTestConnectionFailsWhenNotConfiguredWithoutCallingTheProvider(t *testing.T) {
	f := newFixture(t)

	_, err := application.NewTestCodeHostConnection(f.settings, f.factory).Execute(context.Background(), models.GitHub)

	require.Equal(t, codeerrors.NotConfigured(models.GitHub), err)
	require.Zero(t, f.client.userCalls)
}

func TestTestConnectionUsesTheClientOfTheRequestedProvider(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = []models.CodeHostSettings{githubSettings(t)}
	f.factory.byProvider = map[models.HostingProvider]outbound.CodeHostClient{
		models.GitHub: &fakeClient{user: "octocat"},
	}

	user, err := application.NewTestCodeHostConnection(f.settings, f.factory).Execute(context.Background(), models.GitHub)

	require.NoError(t, err)
	require.Equal(t, "octocat", user)
}

func saveInput(baseURL string, token string) application.SaveCodeHostSettingsInput {
	return application.SaveCodeHostSettingsInput{
		Provider:    models.GitLab,
		BaseURL:     baseURL,
		Token:       token,
		InsecureTLS: true,
	}
}

func TestSaveStoresNormalizedSettings(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = nil

	saved, err := application.NewSaveCodeHostSettings(f.settings).Execute(context.Background(), saveInput("https://git.corp/", " nuevo "))

	require.NoError(t, err)
	require.Equal(t, "nuevo", saved.Token())
	require.Len(t, f.settings.saved, 1)
	require.Equal(t, "https://git.corp", f.settings.saved[0].BaseURL())
	require.Equal(t, "nuevo", f.settings.saved[0].Token())
	require.True(t, f.settings.saved[0].InsecureTLS)
}

func TestSaveBlankTokenKeepsTheStoredOneForTheSameURL(t *testing.T) {
	f := newFixture(t)

	_, err := application.NewSaveCodeHostSettings(f.settings).Execute(context.Background(), saveInput("https://git.corp/", "   "))

	require.NoError(t, err)
	require.Len(t, f.settings.saved, 1)
	require.Equal(t, token, f.settings.saved[0].Token())
	require.Equal(t, "https://git.corp", f.settings.saved[0].BaseURL())
}

func TestSaveBlankTokenWithADifferentURLIsRejectedAndNothingIsSaved(t *testing.T) {
	f := newFixture(t)

	_, err := application.NewSaveCodeHostSettings(f.settings).Execute(context.Background(), saveInput("https://otra.corp", ""))

	require.Equal(t, codeerrors.TokenRequiredForNewURL(), err)
	require.Empty(t, f.settings.saved)
}

func TestSaveMissingTokenWithoutStoredOneIsRejectedAndNothingIsSaved(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = nil

	_, err := application.NewSaveCodeHostSettings(f.settings).Execute(context.Background(), saveInput("https://git.corp", ""))

	require.Equal(t, codeerrors.MissingToken(), err)
	require.Empty(t, f.settings.saved)
}

func TestSaveInvalidURLIsRejectedAndNothingIsSaved(t *testing.T) {
	f := newFixture(t)

	_, err := application.NewSaveCodeHostSettings(f.settings).Execute(context.Background(), saveInput("git.corp", "tok"))

	require.ErrorIs(t, err, codeerrors.InvalidURL(""))
	require.Empty(t, f.settings.saved)
}

func TestSaveStoresEachProviderUnderItsOwnIdentity(t *testing.T) {
	f := newFixture(t)
	f.settings.stored = nil
	input := application.SaveCodeHostSettingsInput{
		Provider: models.GitHub,
		BaseURL:  "https://github.com",
		Token:    "ghp_x",
	}

	saved, err := application.NewSaveCodeHostSettings(f.settings).Execute(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, models.GitHub, saved.Provider())
	require.Equal(t, models.GitHub, f.settings.saved[0].Provider())
}

func TestSaveBlankTokenOnlyReusesTheStoredTokenOfTheSameProvider(t *testing.T) {
	f := newFixture(t)
	input := application.SaveCodeHostSettingsInput{
		Provider: models.GitHub,
		BaseURL:  "https://github.com",
	}

	_, err := application.NewSaveCodeHostSettings(f.settings).Execute(context.Background(), input)

	require.Equal(t, codeerrors.MissingToken(), err)
	require.Empty(t, f.settings.saved)
}

func TestSaveInputMasksTheToken(t *testing.T) {
	input := saveInput("https://git.corp", "s3cr3t-token")

	require.NotContains(t, fmt.Sprintf("%v %+v %#v", input, input, input), "s3cr3t-token")
}
