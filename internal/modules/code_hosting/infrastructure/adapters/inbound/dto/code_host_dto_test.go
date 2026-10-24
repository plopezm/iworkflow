package dto

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/application"
	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
)

func TestSettingsDTONeverCarriesTheToken(t *testing.T) {
	settings, err := models.NewCodeHostSettings(models.GitLab, "https://git.corp", "s3cr3t-token", true)
	require.NoError(t, err)

	result := SettingsFrom(settings)

	require.Equal(t, CodeHostSettingsDTO{
		Provider:        "gitlab",
		BaseURL:         "https://git.corp",
		TokenConfigured: true,
		InsecureTLS:     true,
	}, result)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "s3cr3t-token")
	require.Contains(t, string(encoded), `"token_configured":true`)
}

func TestUnconfiguredSettingsMapToTheEmptyDTO(t *testing.T) {
	require.Equal(t, CodeHostSettingsDTO{Provider: "github"}, UnconfiguredSettings(models.GitHub))
}

func TestSaveDTOMapsToTheUseCaseInput(t *testing.T) {
	input, err := SaveCodeHostSettingsDTO{
		Provider: "github",
		BaseURL:  "https://github.com",
	}.ToInput()

	require.NoError(t, err)
	require.Equal(t, models.GitHub, input.Provider)
	require.Empty(t, input.Token)
	require.Equal(t, "https://github.com", input.BaseURL)
}

func TestSaveDTOWithUnknownProviderIsRejected(t *testing.T) {
	_, err := SaveCodeHostSettingsDTO{Provider: "bitbucket"}.ToInput()

	require.Equal(t, codeerrors.UnknownProvider("bitbucket"), err)
}

func TestSaveDTOStringMasksTheToken(t *testing.T) {
	token := "ghp_visible_in_memory_only"
	form := SaveCodeHostSettingsDTO{Provider: "github", BaseURL: "https://github.com", Token: &token}

	require.NotContains(t, fmt.Sprintf("%v %+v %#v", form, form, form), token)
}

func TestCreateMergeRequestDTOTurnsNilIntoEmptyFields(t *testing.T) {
	title := "Título propio"

	input := CreateMergeRequestDTO{TaskID: "task-1", Title: &title}.ToInput()

	require.Equal(t, application.CreateTaskMergeRequestInput{TaskID: "task-1", Title: "Título propio"}, input)
}

func TestStatusMapsTheMergeRequestAndItsJobs(t *testing.T) {
	status := models.MergeRequestStatus{
		Provider:    models.GitHub,
		ProjectPath: "g/p",
		MergeRequest: models.MergeRequest{
			IID:          7,
			Title:        "T",
			State:        "opened",
			SourceBranch: "pi/x",
			TargetBranch: "main",
			WebURL:       "u",
			MergeStatus:  "mergeable",
		},
		Pipeline: &models.Pipeline{
			ID:     1,
			Status: "success",
			WebURL: "p",
			Jobs: []models.PipelineJob{{
				ID:     2,
				Name:   "build",
				Stage:  "build",
				Status: "success",
				WebURL: "j",
			}},
		},
	}

	result := MergeRequestStatusFrom(status)

	require.Equal(t, "github", result.Provider)
	require.True(t, result.MergeRequest.IsOpen())
	require.NotNil(t, result.Pipeline)
	require.Equal(t, "build", result.Pipeline.Jobs[0].Name)
}

func TestStatusWithoutPipelineSerializesAsNull(t *testing.T) {
	result := MergeRequestStatusFrom(models.MergeRequestStatus{Provider: models.GitLab, ProjectPath: "g/p"})

	encoded, err := json.Marshal(result)

	require.NoError(t, err)
	require.True(t, strings.Contains(string(encoded), `"pipeline":null`), string(encoded))
}
