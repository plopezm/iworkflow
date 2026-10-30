package models_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func TestDraftCarriesKeySummaryURLAndPriority(t *testing.T) {
	draft := models.DraftFromIssue(sampleIssue("ABC-1"))

	assert.Equal(t, "ABC-1: Resumen de ABC-1", draft.Title)
	assert.Contains(t, draft.Prompt, "Jira ABC-1: Resumen de ABC-1")
	assert.Contains(t, draft.Prompt, "https://jira.local/browse/ABC-1")
	assert.Contains(t, draft.Prompt, "Descripción larga")
	assert.Equal(t, models.PriorityHintHigh, draft.Priority)
}

func TestDraftMarksMissingDescriptionAndTruncatesLongTitles(t *testing.T) {
	issue := sampleIssue("ABC-1")
	issue.Remote.Description = "  "
	issue.Remote.Summary = strings.Repeat("x", 300)

	draft := models.DraftFromIssue(issue)

	assert.Contains(t, draft.Prompt, "(sin descripción)")
	assert.Equal(t, 120, utf8.RuneCountInString(draft.Title))
}

func TestDraftWithoutPriorityNamesItAndUsesNormal(t *testing.T) {
	issue := sampleIssue("ABC-1")
	issue.Remote.Priority = nil

	draft := models.DraftFromIssue(issue)

	assert.Contains(t, draft.Prompt, "Prioridad: sin prioridad")
	assert.Equal(t, models.PriorityHintNormal, draft.Priority)
}

func request() models.ConversionRequest {
	base := "  "
	return models.ConversionRequest{
		Key:         "ABC-1",
		WorkspaceID: " ws ",
		BaseBranch:  &base,
		Title:       " t ",
		Prompt:      " p ",
		Priority:    models.PriorityHintNormal,
		StartNow:    false,
	}
}

func TestValidatedTrimsAndDropsBlankBranch(t *testing.T) {
	valid, err := request().Validated()
	require.NoError(t, err)

	assert.Equal(t, "ws", valid.WorkspaceID)
	assert.Equal(t, "t", valid.Title)
	assert.Equal(t, "p", valid.Prompt)
	assert.Nil(t, valid.BaseBranch)
}

func TestValidatedKeepsTheTrimmedBranch(t *testing.T) {
	branch := "  origin/develop "
	input := request()
	input.BaseBranch = &branch

	valid, err := input.Validated()
	require.NoError(t, err)
	require.NotNil(t, valid.BaseBranch)
	assert.Equal(t, "origin/develop", *valid.BaseBranch)
}

func TestValidatedRejectsMissingFields(t *testing.T) {
	noWorkspace := request()
	noWorkspace.WorkspaceID = " "
	_, err := noWorkspace.Validated()
	require.ErrorIs(t, err, jiraerrors.MissingWorkspace())

	noTitle := request()
	noTitle.Title = " "
	_, err = noTitle.Validated()
	require.ErrorIs(t, err, jiraerrors.EmptyTitle())

	noPrompt := request()
	noPrompt.Prompt = " "
	_, err = noPrompt.Validated()
	require.ErrorIs(t, err, jiraerrors.EmptyPrompt())
}

func TestValidatedTruncatesTheTitle(t *testing.T) {
	input := request()
	input.Title = strings.Repeat("ñ", 130)

	valid, err := input.Validated()
	require.NoError(t, err)
	assert.Equal(t, 120, utf8.RuneCountInString(valid.Title))
}
