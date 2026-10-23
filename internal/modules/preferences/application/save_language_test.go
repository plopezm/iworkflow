package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
)

func TestSaveLanguageStoresASupportedLanguage(t *testing.T) {
	repository := newFakeRepository()

	saved, err := application.NewSaveLanguage(repository).Execute(context.Background(), "es")

	require.NoError(t, err)
	require.Equal(t, "es", saved.Code())
	require.NotNil(t, repository.language)
	require.Equal(t, "es", repository.language.Code())
	require.Equal(t, 1, repository.writes)
}

func TestSaveLanguageRejectsUnsupportedCodesWithoutTouchingStorage(t *testing.T) {
	repository := newFakeRepository()

	_, err := application.NewSaveLanguage(repository).Execute(context.Background(), "pt")

	require.EqualError(t, err, "idioma no soportado: pt")
	require.Equal(t, 0, repository.writes)
	require.Nil(t, repository.language)
}

func TestSaveLanguagePropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.writeErr = errStorage

	_, err := application.NewSaveLanguage(repository).Execute(context.Background(), "de")

	require.ErrorIs(t, err, errStorage)
	var unsupported *domainerrors.UnsupportedLanguageError
	require.NotErrorAs(t, err, &unsupported)
}
