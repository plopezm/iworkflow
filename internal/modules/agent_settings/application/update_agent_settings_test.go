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

func validModel() models.ModelRef {
	return models.ModelRef{Provider: "anthropic", Model: "claude-5"}
}

func TestUpdateAgentSettingsSavesWhenModelAndProfileAreValid(t *testing.T) {
	repository := &fakeRepository{}
	model := validModel()
	modelCatalog := &fakeModelCatalog{models: []models.ModelRef{model}}
	profileCatalog := &fakeProfileCatalog{catalog: models.ProfileCatalog{Names: []string{"Claude"}}}
	useCase := application.NewUpdateAgentSettings(repository, modelCatalog, profileCatalog)
	settings := models.AgentSettings{
		Model:    &model,
		Profile:  profileName(t, "Claude"),
		Thinking: thinkingPointer(models.ThinkingHigh),
	}

	result, err := useCase.Execute(context.Background(), settings)

	require.NoError(t, err)
	require.Equal(t, settings, result)
	require.Equal(t, []models.AgentSettings{settings}, repository.saved)
}

func TestUpdateAgentSettingsRejectsModelNotInCatalog(t *testing.T) {
	repository := &fakeRepository{}
	useCase := application.NewUpdateAgentSettings(repository, &fakeModelCatalog{}, &fakeProfileCatalog{})
	model := validModel()

	result, err := useCase.Execute(context.Background(), models.AgentSettings{Model: &model})

	require.EqualError(t, err, "modelo no disponible: anthropic/claude-5")
	require.ErrorIs(t, err, agenterrors.ErrModelNotFound)
	require.Equal(t, models.AgentSettings{}, result)
	require.Empty(t, repository.saved)
}

func TestUpdateAgentSettingsRejectsProfileNotInStore(t *testing.T) {
	repository := &fakeRepository{}
	profileCatalog := &fakeProfileCatalog{catalog: models.ProfileCatalog{Names: []string{}}}
	useCase := application.NewUpdateAgentSettings(repository, &fakeModelCatalog{}, profileCatalog)

	_, err := useCase.Execute(context.Background(), models.AgentSettings{Profile: profileName(t, "Ghost")})

	require.EqualError(t, err, "perfil no encontrado: Ghost")
	require.Empty(t, repository.saved)
}

func TestUpdateAgentSettingsSavesThinkingWithoutTouchingCatalogs(t *testing.T) {
	repository := &fakeRepository{}
	modelCatalog := &fakeModelCatalog{}
	profileCatalog := &fakeProfileCatalog{}
	useCase := application.NewUpdateAgentSettings(repository, modelCatalog, profileCatalog)
	settings := models.AgentSettings{Thinking: thinkingPointer(models.ThinkingMax)}

	result, err := useCase.Execute(context.Background(), settings)

	require.NoError(t, err)
	require.Equal(t, settings, result)
	require.Empty(t, modelCatalog.requests)
	require.Zero(t, profileCatalog.calls)
}

func TestUpdateAgentSettingsSavesDefaultSettingsWithoutTouchingCatalogs(t *testing.T) {
	repository := &fakeRepository{}
	modelCatalog := &fakeModelCatalog{}
	profileCatalog := &fakeProfileCatalog{}
	useCase := application.NewUpdateAgentSettings(repository, modelCatalog, profileCatalog)

	result, err := useCase.Execute(context.Background(), models.AgentSettings{})

	require.NoError(t, err)
	require.Equal(t, models.AgentSettings{}, result)
	require.Equal(t, []models.AgentSettings{{}}, repository.saved)
	require.Empty(t, modelCatalog.requests)
	require.Zero(t, profileCatalog.calls)
}

func TestUpdateAgentSettingsValidatesModelAgainstTheSelectedRuntimeCatalog(t *testing.T) {
	modelCatalog := &fakeModelCatalog{}
	useCase := application.NewUpdateAgentSettings(&fakeRepository{}, modelCatalog, &fakeProfileCatalog{})
	model := validModel()

	_, err := useCase.Execute(context.Background(), models.AgentSettings{
		Runtime: runtimePointer(models.RuntimePi),
		Model:   &model,
	})

	require.ErrorIs(t, err, agenterrors.ErrModelNotFound)
	require.Equal(t, []models.AgentRuntime{models.RuntimePi}, modelCatalog.requests)
}

func TestUpdateAgentSettingsDropsThinkingUnsupportedByClaude(t *testing.T) {
	repository := &fakeRepository{}
	useCase := application.NewUpdateAgentSettings(repository, &fakeModelCatalog{}, &fakeProfileCatalog{})

	result, err := useCase.Execute(context.Background(), models.AgentSettings{
		Runtime:  runtimePointer(models.RuntimeClaude),
		Thinking: thinkingPointer(models.ThinkingMinimal),
	})

	require.NoError(t, err)
	require.Nil(t, result.Thinking)
	require.Len(t, repository.saved, 1)
	require.Nil(t, repository.saved[0].Thinking)
}

func TestUpdateAgentSettingsKeepsThinkingCompatibleWithClaude(t *testing.T) {
	useCase := application.NewUpdateAgentSettings(&fakeRepository{}, &fakeModelCatalog{}, &fakeProfileCatalog{})

	result, err := useCase.Execute(context.Background(), models.AgentSettings{
		Runtime:  runtimePointer(models.RuntimeClaude),
		Thinking: thinkingPointer(models.ThinkingXHigh),
	})

	require.NoError(t, err)
	require.Equal(t, thinkingPointer(models.ThinkingXHigh), result.Thinking)
}

func TestUpdateAgentSettingsKeepsEveryLevelForPiAndTheDefaultRuntime(t *testing.T) {
	for _, runtime := range []*models.AgentRuntime{nil, runtimePointer(models.RuntimePi), runtimePointer(models.RuntimeGentleShell)} {
		repository := &fakeRepository{}
		useCase := application.NewUpdateAgentSettings(repository, &fakeModelCatalog{}, &fakeProfileCatalog{})

		result, err := useCase.Execute(context.Background(), models.AgentSettings{
			Runtime:  runtime,
			Thinking: thinkingPointer(models.ThinkingOff),
		})

		require.NoError(t, err)
		require.Equal(t, thinkingPointer(models.ThinkingOff), result.Thinking)
	}
}

func TestUpdateAgentSettingsPropagatesSaveError(t *testing.T) {
	repository := &fakeRepository{saveErr: agenterrors.Repository(errors.New("locked"))}
	useCase := application.NewUpdateAgentSettings(repository, &fakeModelCatalog{}, &fakeProfileCatalog{})

	_, err := useCase.Execute(context.Background(), models.AgentSettings{})

	require.ErrorIs(t, err, agenterrors.ErrRepository)
}
