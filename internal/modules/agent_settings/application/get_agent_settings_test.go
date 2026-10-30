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

func TestGetAgentSettingsReturnsSettingsFromRepository(t *testing.T) {
	model := models.ModelRef{Provider: "anthropic", Model: "claude-5"}
	repository := &fakeRepository{settings: models.AgentSettings{Model: &model}}
	useCase := application.NewGetAgentSettings(repository)

	result, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result.Model)
	require.Equal(t, model, *result.Model)
}

func TestGetAgentSettingsPropagatesRepositoryError(t *testing.T) {
	repository := &fakeRepository{getErr: agenterrors.Repository(errors.New("disk full"))}
	useCase := application.NewGetAgentSettings(repository)

	_, err := useCase.Execute(context.Background())

	require.ErrorIs(t, err, agenterrors.ErrRepository)
	require.EqualError(t, err, "error de persistencia: disk full")
}
