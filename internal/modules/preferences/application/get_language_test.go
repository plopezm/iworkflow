package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
	"iworkflow/internal/modules/preferences/domain/models"
)

func TestGetLanguageReturnsTheStoredLanguage(t *testing.T) {
	repository := newFakeRepository()
	french, err := models.ParseUILanguage("fr")
	require.NoError(t, err)
	repository.language = &french

	language, err := application.NewGetLanguage(repository).Execute(context.Background())

	require.NoError(t, err)
	require.NotNil(t, language)
	require.Equal(t, "fr", language.Code())
}

func TestGetLanguageReturnsNilBeforeAnyChoice(t *testing.T) {
	language, err := application.NewGetLanguage(newFakeRepository()).Execute(context.Background())

	require.NoError(t, err)
	require.Nil(t, language)
}

func TestGetLanguagePropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.readErr = errStorage

	_, err := application.NewGetLanguage(repository).Execute(context.Background())

	require.ErrorIs(t, err, errStorage)
}
