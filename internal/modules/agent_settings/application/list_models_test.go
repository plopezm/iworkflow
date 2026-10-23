package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/application"
	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

func TestListModelsUsesGentleShellWhenNoRuntimeIsSaved(t *testing.T) {
	catalog := &fakeModelCatalog{models: []models.ModelRef{{Provider: "anthropic", Model: "claude-5"}}}
	useCase := application.NewListModels(&fakeRepository{}, catalog)

	result, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, []models.AgentRuntime{models.RuntimeGentleShell}, catalog.requests)
}

func TestListModelsQueriesTheCatalogOfTheSelectedRuntime(t *testing.T) {
	pi := models.RuntimePi
	catalog := &fakeModelCatalog{}
	useCase := application.NewListModels(&fakeRepository{settings: models.AgentSettings{Runtime: &pi}}, catalog)

	_, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, []models.AgentRuntime{models.RuntimePi}, catalog.requests)
}

func TestListModelsPropagatesCatalogError(t *testing.T) {
	catalog := &fakeModelCatalog{err: agenterrors.Catalog("boom")}
	useCase := application.NewListModels(&fakeRepository{}, catalog)

	result, err := useCase.Execute(context.Background())

	require.Nil(t, result)
	require.EqualError(t, err, "error de catálogo: boom")
	require.ErrorIs(t, err, agenterrors.ErrCatalog)
}

func TestListModelsPropagatesRepositoryErrorWithoutQueryingTheCatalog(t *testing.T) {
	catalog := &fakeModelCatalog{}
	repository := &fakeRepository{getErr: agenterrors.Repository(errors.New("locked"))}
	useCase := application.NewListModels(repository, catalog)

	_, err := useCase.Execute(context.Background())

	require.ErrorIs(t, err, agenterrors.ErrRepository)
	require.Empty(t, catalog.requests)
}
