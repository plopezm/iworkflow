package code_hosting_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/persistence"
)

func newSettings(t *testing.T, provider models.HostingProvider, baseURL string, token string, insecure bool) models.CodeHostSettings {
	t.Helper()
	settings, err := models.NewCodeHostSettings(provider, baseURL, token, insecure)
	require.NoError(t, err)
	return settings
}

func TestSettingsRepositoryReturnsNilWhenTheRowIsMissing(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewCodeHostSettingsRepository(dbtest.New(t))

	settings, err := repository.Get(ctx, models.GitLab)

	require.NoError(t, err)
	require.Nil(t, settings)
}

func TestSettingsRepositorySavesAndReadsBackTheSameSettings(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewCodeHostSettingsRepository(dbtest.New(t))
	original := newSettings(t, models.GitLab, "https://git.corp/", "tok-1", true)

	require.NoError(t, repository.Save(ctx, original))
	loaded, err := repository.Get(ctx, models.GitLab)

	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, original, *loaded)
}

func TestSettingsRepositorySaveOverwritesThePreviousRow(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewCodeHostSettingsRepository(dbtest.New(t))
	first := newSettings(t, models.GitLab, "https://one.corp", "tok-1", false)
	second := newSettings(t, models.GitLab, "https://two.corp", "tok-2", false)

	require.NoError(t, repository.Save(ctx, first))
	require.NoError(t, repository.Save(ctx, second))
	loaded, err := repository.Get(ctx, models.GitLab)

	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, second, *loaded)
}

func TestSettingsRepositoryStoresProvidersIndependently(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewCodeHostSettingsRepository(dbtest.New(t))
	gitlab := newSettings(t, models.GitLab, "https://git.corp", "tok-gl", false)
	github := newSettings(t, models.GitHub, "https://github.com", "tok-gh", false)

	require.NoError(t, repository.Save(ctx, gitlab))
	require.NoError(t, repository.Save(ctx, github))

	gotGitHub, err := repository.Get(ctx, models.GitHub)
	require.NoError(t, err)
	require.NotNil(t, gotGitHub)
	require.Equal(t, github, *gotGitHub)
	gotGitLab, err := repository.Get(ctx, models.GitLab)
	require.NoError(t, err)
	require.NotNil(t, gotGitLab)
	require.Equal(t, gitlab, *gotGitLab)
	listed, err := repository.List(ctx)
	require.NoError(t, err)
	require.Equal(t, []models.CodeHostSettings{github, gitlab}, listed)
}

func TestSettingsRepositoryReturnsNilForAnUnconfiguredProviderWhileAnotherIsStored(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewCodeHostSettingsRepository(dbtest.New(t))
	require.NoError(t, repository.Save(ctx, newSettings(t, models.GitLab, "https://git.corp", "tok", false)))

	settings, err := repository.Get(ctx, models.GitHub)

	require.NoError(t, err)
	require.Nil(t, settings)
}
