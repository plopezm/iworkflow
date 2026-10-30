package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
)

func TestHostingProviderIdentifiersRoundTripThroughParse(t *testing.T) {
	for _, provider := range models.AllHostingProviders() {
		parsed, err := models.ParseHostingProvider(string(provider))
		require.NoError(t, err)
		require.Equal(t, provider, parsed)
	}
}

func TestHostingProviderParseIgnoresCaseAndWhitespace(t *testing.T) {
	parsed, err := models.ParseHostingProvider(" GitHub ")
	require.NoError(t, err)
	require.Equal(t, models.GitHub, parsed)
}

func TestHostingProviderParseRejectsUnknownProvider(t *testing.T) {
	_, err := models.ParseHostingProvider("bitbucket")
	require.ErrorIs(t, err, codeerrors.UnknownProvider("bitbucket"))
	require.Equal(t, "proveedor de repositorios desconocido: bitbucket", err.Error())
}

func TestHostingProviderLabelsAreUserFacing(t *testing.T) {
	require.Equal(t, "GitLab", models.GitLab.Label())
	require.Equal(t, "GitHub", models.GitHub.Label())
}
