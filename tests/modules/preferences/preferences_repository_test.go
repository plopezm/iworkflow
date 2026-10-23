package preferences_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/outbound/persistence"
)

// insertRaw writes a value the application would never write, to simulate damaged or
// newer-version data.
func insertRaw(t *testing.T, db *sql.DB, key string, value string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		"INSERT INTO app_preferences (key, value) VALUES (?, ?)", key, value)
	require.NoError(t, err)
}

func mustUILanguage(t *testing.T, code string) models.UILanguage {
	t.Helper()
	language, err := models.ParseUILanguage(code)
	require.NoError(t, err)
	return language
}

func TestLanguageIsNilBeforeAnyChoice(t *testing.T) {
	repository := persistence.NewPreferencesRepository(dbtest.New(t))

	language, err := repository.Language(context.Background())

	require.NoError(t, err)
	require.Nil(t, language)
}

func TestLanguageSaveReadRoundtripAndOverwrite(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewPreferencesRepository(dbtest.New(t))

	require.NoError(t, repository.SaveLanguage(ctx, mustUILanguage(t, "es")))
	language, err := repository.Language(ctx)
	require.NoError(t, err)
	require.Equal(t, "es", language.Code())

	require.NoError(t, repository.SaveLanguage(ctx, mustUILanguage(t, "zh")))
	language, err = repository.Language(ctx)
	require.NoError(t, err)
	require.Equal(t, "zh", language.Code())
}

func TestUnknownStoredLanguageCountsAsUnset(t *testing.T) {
	db := dbtest.New(t)
	insertRaw(t, db, "language", "xx")
	repository := persistence.NewPreferencesRepository(db)

	language, err := repository.Language(context.Background())

	require.NoError(t, err)
	require.Nil(t, language)
}

func TestNetworkAccessIsOffBeforeAnyChoice(t *testing.T) {
	repository := persistence.NewPreferencesRepository(dbtest.New(t))

	enabled, err := repository.NetworkAccess(context.Background())

	require.NoError(t, err)
	require.False(t, enabled)
}

func TestNetworkAccessRoundtripDoesNotTouchTheLanguage(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewPreferencesRepository(dbtest.New(t))
	require.NoError(t, repository.SaveLanguage(ctx, mustUILanguage(t, "es")))

	require.NoError(t, repository.SaveNetworkAccess(ctx, true))
	enabled, err := repository.NetworkAccess(ctx)
	require.NoError(t, err)
	require.True(t, enabled)
	language, err := repository.Language(ctx)
	require.NoError(t, err)
	require.Equal(t, "es", language.Code())

	require.NoError(t, repository.SaveNetworkAccess(ctx, false))
	enabled, err = repository.NetworkAccess(ctx)
	require.NoError(t, err)
	require.False(t, enabled)
}

func TestUnknownNetworkAccessValueKeepsTheServerPrivate(t *testing.T) {
	db := dbtest.New(t)
	insertRaw(t, db, "network_access", "yes")
	repository := persistence.NewPreferencesRepository(db)

	enabled, err := repository.NetworkAccess(context.Background())

	require.NoError(t, err)
	require.False(t, enabled)
}

func TestAccessTokenIsNilBeforeOneIsGenerated(t *testing.T) {
	repository := persistence.NewPreferencesRepository(dbtest.New(t))

	token, err := repository.AccessToken(context.Background())

	require.NoError(t, err)
	require.Nil(t, token)
}

func TestAccessTokenRoundtripAndReplacement(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewPreferencesRepository(dbtest.New(t))
	first, err := models.GenerateAccessToken()
	require.NoError(t, err)
	second, err := models.GenerateAccessToken()
	require.NoError(t, err)

	require.NoError(t, repository.SaveAccessToken(ctx, first))
	stored, err := repository.AccessToken(ctx)
	require.NoError(t, err)
	require.Equal(t, first, *stored)

	require.NoError(t, repository.SaveAccessToken(ctx, second))
	stored, err = repository.AccessToken(ctx)
	require.NoError(t, err)
	require.Equal(t, second, *stored)
}

func TestDamagedAccessTokenCountsAsMissing(t *testing.T) {
	db := dbtest.New(t)
	insertRaw(t, db, "access_token", "short")
	repository := persistence.NewPreferencesRepository(db)

	token, err := repository.AccessToken(context.Background())

	require.NoError(t, err)
	require.Nil(t, token)
}

func TestTokenIsRequiredUnlessExplicitlySwitchedOff(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewPreferencesRepository(dbtest.New(t))

	required, err := repository.RequireAccessToken(ctx)
	require.NoError(t, err)
	require.True(t, required, "default")

	require.NoError(t, repository.SaveRequireAccessToken(ctx, false))
	required, err = repository.RequireAccessToken(ctx)
	require.NoError(t, err)
	require.False(t, required, "off")

	require.NoError(t, repository.SaveRequireAccessToken(ctx, true))
	required, err = repository.RequireAccessToken(ctx)
	require.NoError(t, err)
	require.True(t, required, "on")
}

func TestUnknownRequireTokenValueKeepsTheTokenRequired(t *testing.T) {
	db := dbtest.New(t)
	insertRaw(t, db, "require_access_token", "maybe")
	repository := persistence.NewPreferencesRepository(db)

	required, err := repository.RequireAccessToken(context.Background())

	require.NoError(t, err)
	require.True(t, required)
}

func TestListenPortIsNilBeforeOneIsChosen(t *testing.T) {
	repository := persistence.NewPreferencesRepository(dbtest.New(t))

	port, err := repository.ListenPort(context.Background())

	require.NoError(t, err)
	require.Nil(t, port)
}

func TestListenPortRoundtripAndReplacement(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewPreferencesRepository(dbtest.New(t))
	first, err := models.NewListenPort(9000)
	require.NoError(t, err)
	second, err := models.NewListenPort(9100)
	require.NoError(t, err)

	require.NoError(t, repository.SaveListenPort(ctx, first))
	stored, err := repository.ListenPort(ctx)
	require.NoError(t, err)
	require.Equal(t, first, *stored)

	require.NoError(t, repository.SaveListenPort(ctx, second))
	stored, err = repository.ListenPort(ctx)
	require.NoError(t, err)
	require.Equal(t, second, *stored)
}

func TestDamagedListenPortCountsAsUnset(t *testing.T) {
	db := dbtest.New(t)
	insertRaw(t, db, "listen_port", "80")
	repository := persistence.NewPreferencesRepository(db)

	port, err := repository.ListenPort(context.Background())

	require.NoError(t, err)
	require.Nil(t, port)
}

func TestStoredPortWithPlusSignIsReadBack(t *testing.T) {
	db := dbtest.New(t)
	insertRaw(t, db, "listen_port", "+9100")
	repository := persistence.NewPreferencesRepository(db)

	port, err := repository.ListenPort(context.Background())

	require.NoError(t, err)
	require.NotNil(t, port)
	require.Equal(t, uint16(9100), port.Port())
}

func TestStorageFailureIsReportedAsARepositoryError(t *testing.T) {
	db := dbtest.New(t)
	require.NoError(t, db.Close())
	repository := persistence.NewPreferencesRepository(db)

	_, err := repository.Language(context.Background())

	require.Error(t, err)
	require.ErrorContains(t, err, "error de almacenamiento: ")
}
