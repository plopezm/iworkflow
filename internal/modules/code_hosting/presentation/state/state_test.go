package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	dto "iworkflow/internal/modules/code_hosting/infrastructure/adapters/inbound/dto"
)

func spanish() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es})
}

func TestProviderVocabularyFollowsTheProvider(t *testing.T) {
	require.Equal(t, "GitHub", providerLabel("github"))
	require.Equal(t, "PR", requestAbbrev("github"))
	require.Equal(t, "#", requestPrefix("github"))
	require.Equal(t, "Workflow", pipelineNoun("github"))
	require.Equal(t, "GitLab", providerLabel("gitlab"))
	require.Equal(t, "MR", requestAbbrev("gitlab"))
	require.Equal(t, "!", requestPrefix("gitlab"))
	require.Equal(t, "Pipeline", pipelineNoun("gitlab"))
}

func TestUnknownProviderUsesTheGitLabVocabulary(t *testing.T) {
	for _, provider := range []string{"gitlab", "", "otro"} {
		require.Equal(t, "GitLab", providerLabel(provider))
		require.Equal(t, "MR", requestAbbrev(provider))
	}
}

func TestPlaceholdersAndHelpAreProviderSpecific(t *testing.T) {
	require.NotEqual(t, urlPlaceholder("github"), urlPlaceholder("gitlab"))
	require.NotEqual(t, tokenPlaceholder("github"), tokenPlaceholder("gitlab"))
	require.Contains(t, providerHelp(context.Background(), "github"), "pull requests")
	require.Contains(t, providerHelp(context.Background(), "gitlab"), "merge requests")
}

func TestGitHubFormStartsWithGitHubComOnlyWhenNothingIsStored(t *testing.T) {
	require.Equal(t, "https://github.com", initialBaseURL("github", ""))
	require.Equal(t, "https://github.com", initialBaseURL("github", "  "))
	require.Equal(t, "https://ghe.corp", initialBaseURL("github", "https://ghe.corp"))
	require.Equal(t, "", initialBaseURL("gitlab", ""))
}

func TestStatusVariantsAreSemantic(t *testing.T) {
	require.Equal(t, "ok", statusVariant("success"))
	require.Equal(t, "error", statusVariant("failed"))
	require.Equal(t, "info", statusVariant("running"))
	require.Equal(t, "warn", statusVariant("manual"))
	require.Equal(t, "muted", statusVariant("canceled"))
	require.Equal(t, "muted", statusVariant("algo-nuevo"))
}

func TestStatusLabelsAreTranslatedAndUnknownOnesPassThrough(t *testing.T) {
	require.Equal(t, "failed", statusLabel(context.Background(), "failed"))
	require.Equal(t, "merged", statusLabel(context.Background(), "merged"))
	require.Equal(t, "fallido", statusLabel(spanish(), "failed"))
	require.Equal(t, "algo-nuevo", statusLabel(spanish(), "algo-nuevo"))
}

func TestSettingsCardNeverCarriesTheToken(t *testing.T) {
	stored := dto.CodeHostSettingsDTO{Provider: "gitlab", BaseURL: "https://git.corp", TokenConfigured: true}

	props := SettingsCard(context.Background(), "gitlab", SettingsCardData{Stored: &stored})

	require.Equal(t, "https://git.corp", props.BaseURL)
	require.Equal(t, "•••••• (leave empty to keep it)", props.TokenPlaceholder)
	require.True(t, props.TokenConfigured)
	require.Equal(t, "code-host-settings-gitlab", props.ID)
	require.Equal(t, "/code-host/settings/gitlab", props.SaveURL)
	require.Equal(t, "/code-host/settings/gitlab/test", props.TestURL)
	require.Equal(t, "GitLab URL", props.BaseURLLabel)
}

func TestSettingsCardOffersTheProviderExampleWhenNothingIsStored(t *testing.T) {
	props := SettingsCard(context.Background(), "gitlab", SettingsCardData{})

	require.False(t, props.TokenConfigured)
	require.Equal(t, "glpat-…", props.TokenPlaceholder)
	require.Empty(t, props.BaseURL)
}

func TestSettingsCardKeepsWhatTheUserTypedWhenASaveFails(t *testing.T) {
	stored := dto.CodeHostSettingsDTO{Provider: "gitlab", BaseURL: "https://git.corp", TokenConfigured: true}

	props := SettingsCard(context.Background(), "gitlab", SettingsCardData{
		Stored:  &stored,
		Entered: &EnteredSettings{BaseURL: "git.typo", InsecureTLS: true},
		SaveErr: "URL inválida: git.typo",
	})

	require.Equal(t, "git.typo", props.BaseURL)
	require.True(t, props.InsecureTLS)
	require.Equal(t, "URL inválida: git.typo", props.SaveError)
}

func TestSettingsCardReplacesTheFormWhenTheStoredSettingsCannotBeRead(t *testing.T) {
	props := SettingsCard(context.Background(), "github", SettingsCardData{LoadErr: "error de persistencia: x"})

	require.Equal(t, "error de persistencia: x", props.LoadError)
	require.Equal(t, "https://github.com", props.BaseURL)
}

func TestConnectionOutcomeShowsTheUserOrTheFailure(t *testing.T) {
	ok := ConnectionOutcome(spanish(), "pablo", "")
	require.True(t, ok.OK)
	require.Equal(t, "Conectado como pablo", ok.Text)

	failed := ConnectionOutcome(context.Background(), "", "GitLab respondió 401: 401 Unauthorized")
	require.False(t, failed.OK)
	require.Equal(t, "GitLab respondió 401: 401 Unauthorized", failed.Text)
}

func mergeRequestDTO(state string, mergeStatus string) dto.MergeRequestDTO {
	return dto.MergeRequestDTO{
		IID:          7,
		Title:        "Mi tarea",
		State:        state,
		SourceBranch: "pi/feature",
		TargetBranch: "main",
		WebURL:       "https://git.corp/g/p/-/merge_requests/7",
		MergeStatus:  mergeStatus,
	}
}

func statusDTO(request dto.MergeRequestDTO, pipeline *dto.PipelineDTO) dto.MergeRequestStatusDTO {
	return dto.MergeRequestStatusDTO{Provider: "gitlab", ProjectPath: "g/p", MergeRequest: request, Pipeline: pipeline}
}

func TestPanelWithoutRequestOffersToOpenIt(t *testing.T) {
	props := MergeRequestPanel(context.Background(), PanelData{TaskID: "task-1", NoRequest: true, Title: "borrador"})

	require.Nil(t, props.Status)
	require.NotNil(t, props.Create)
	require.Equal(t, "borrador", props.Create.Title)
	require.Equal(t, "Create merge/pull request", props.Create.Action)
	require.Equal(t, "This branch has no merge/pull request yet.", props.Create.Intro)
	require.Equal(t, "If the branch is not on origin yet, it is pushed automatically.", props.Create.Note)
}

func TestPanelOnAReadErrorShowsOnlyTheError(t *testing.T) {
	props := MergeRequestPanel(context.Background(), PanelData{TaskID: "task-1", Err: "no coincide"})

	require.Equal(t, "no coincide", props.Error)
	require.Nil(t, props.Create)
	require.Nil(t, props.Status)
}

func TestOpenRequestOffersMergeAndPush(t *testing.T) {
	status := statusDTO(mergeRequestDTO("opened", "mergeable"), nil)

	props := MergeRequestPanel(context.Background(), PanelData{TaskID: "task-1", Status: &status})

	require.NotNil(t, props.Status)
	require.True(t, props.Status.Open)
	require.True(t, props.Status.ShowMergeStatus)
	require.False(t, props.Status.Confirm)
	require.Equal(t, "!7 Mi tarea", props.Status.Heading)
	require.Equal(t, "pi/feature → main · g/p", props.Status.Branches)
	require.Equal(t, "Merge MR", props.Status.MergeButton)
	require.Equal(t, "Push branch", props.Status.PushLabel)
	require.Equal(t, "Open MR in GitLab", props.Status.OpenLabel)
	require.Equal(t, "No CI pipeline.", props.Status.NoPipeline)
}

func TestConfirmingReplacesTheMergeButtonOfAnOpenRequest(t *testing.T) {
	status := statusDTO(mergeRequestDTO("opened", "mergeable"), nil)

	props := MergeRequestPanel(context.Background(), PanelData{TaskID: "task-1", Status: &status, Confirming: true})

	require.True(t, props.Status.Confirm)
	require.Equal(t, "Merge the MR into GitLab? The target branch will be updated on the server.", props.Status.ConfirmMessage)
}

func TestMergedRequestOffersNoActions(t *testing.T) {
	status := statusDTO(mergeRequestDTO("merged", "checking"), nil)

	props := MergeRequestPanel(context.Background(), PanelData{TaskID: "task-1", Status: &status, Confirming: true})

	require.False(t, props.Status.Open)
	require.False(t, props.Status.Confirm)
	require.False(t, props.Status.ShowMergeStatus)
	require.Equal(t, "merged", props.Status.State.Text)
}

func TestPushedNoticeIsTranslatedAndShown(t *testing.T) {
	props := MergeRequestPanel(spanish(), PanelData{TaskID: "task-1", NoRequest: true, Notice: NoticePushed})

	require.Equal(t, "Rama subida a origin.", props.Notice)
}

func TestPipelineShowsItsJobsWithTheProviderNoun(t *testing.T) {
	pipeline := &dto.PipelineDTO{
		ID:     22,
		Status: "failed",
		WebURL: "http://gl/p/22",
		Jobs:   []dto.PipelineJobDTO{{ID: 1, Name: "build", Stage: "build", Status: "success", WebURL: "http://gl/j/1"}},
	}
	status := statusDTO(mergeRequestDTO("opened", "mergeable"), pipeline)

	props := MergeRequestPanel(context.Background(), PanelData{TaskID: "task-1", Status: &status})

	require.NotNil(t, props.Status.Pipeline)
	require.Equal(t, "Pipeline #22", props.Status.Pipeline.Title)
	require.Equal(t, "failed", props.Status.Pipeline.Status.Text)
	require.Equal(t, "(build)", props.Status.Pipeline.Jobs[0].StageLabel)
	require.Equal(t, "ok", props.Status.Pipeline.Jobs[0].Status.Variant)
	require.Equal(t, "successful", props.Status.Pipeline.Jobs[0].Status.Text)
}
