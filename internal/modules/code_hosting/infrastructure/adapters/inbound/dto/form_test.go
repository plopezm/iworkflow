package dto

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaveSettingsFromFormReadsEveryField(t *testing.T) {
	form := url.Values{"base_url": {"https://git.corp"}, "token": {"tok"}, "insecure_tls": {"on"}}

	dto := SaveSettingsFromForm("gitlab", form)

	require.Equal(t, "gitlab", dto.Provider)
	require.Equal(t, "https://git.corp", dto.BaseURL)
	require.NotNil(t, dto.Token)
	require.Equal(t, "tok", *dto.Token)
	require.True(t, dto.InsecureTLS)
}

func TestSaveSettingsFromFormTreatsAnUncheckedBoxAsFalse(t *testing.T) {
	dto := SaveSettingsFromForm("github", url.Values{"base_url": {"https://github.com"}})

	require.False(t, dto.InsecureTLS)
	require.Equal(t, "", *dto.Token)
}

func TestCreateMergeRequestFromFormKeepsTheTitleForTheTask(t *testing.T) {
	dto := CreateMergeRequestFromForm("task-1", url.Values{"title": {"Mi título"}})

	input := dto.ToInput()

	require.Equal(t, "task-1", input.TaskID)
	require.Equal(t, "Mi título", input.Title)
}
