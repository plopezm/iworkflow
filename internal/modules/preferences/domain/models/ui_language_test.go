package models_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/domain/models"
)

func TestParseUILanguageAcceptsEverySupportedCodeAndTrims(t *testing.T) {
	for _, code := range []string{"en", "es", "zh", "de", "fr"} {
		language, err := models.ParseUILanguage(code)
		require.NoError(t, err, code)
		require.Equal(t, code, language.Code())
	}

	language, err := models.ParseUILanguage(" de ")
	require.NoError(t, err)
	require.Equal(t, "de", language.Code())
}

func TestParseUILanguageRejectsUnknownCodes(t *testing.T) {
	for _, code := range []string{"pt", "", "EN", "english"} {
		_, err := models.ParseUILanguage(code)

		var unsupported *domainerrors.UnsupportedLanguageError
		require.ErrorAs(t, err, &unsupported, code)
		require.Equal(t, code, unsupported.Code)
	}
}

func TestUnsupportedLanguageMessageIsTheParityText(t *testing.T) {
	_, err := models.ParseUILanguage("pt")

	require.EqualError(t, err, "idioma no soportado: pt")
	require.False(t, errors.Is(err, domainerrors.ErrInvalidAccessToken))
}
