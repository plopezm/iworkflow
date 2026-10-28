package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/state"
)

func TestConvertIssueViewAndBranchFormatting(t *testing.T) {
	localBranch := dto.ConversionBranchDTO{Name: "feature/foo", IsRemote: false}
	remoteBranch := dto.ConversionBranchDTO{Name: "origin/feature/bar", IsRemote: true}

	view := state.ConvertIssueView{
		Preparation: dto.ConversionPreparationDTO{
			Issue:    dto.IssueDTO{Key: "DEV-123"},
			Title:    "Draft task",
			Prompt:   "Draft prompt",
			Priority: "high",
		},
		Branches: []dto.ConversionBranchDTO{localBranch, remoteBranch},
		Error:    "",
	}

	assert.Equal(t, "DEV-123", view.Preparation.Issue.Key)
	assert.Len(t, view.Branches, 2)
	assert.Nil(t, view.Converted)
	assert.Empty(t, view.Error)

	// Branch formatting
	assert.Equal(t, "feature/foo", state.FormatBranch(localBranch, i18n.Es))
	assert.Equal(t, "feature/foo", state.FormatBranch(localBranch, i18n.En))
	assert.Equal(t, "origin/feature/bar (remota)", state.FormatBranch(remoteBranch, i18n.Es))
	assert.Equal(t, "origin/feature/bar (remote)", state.FormatBranch(remoteBranch, i18n.En))

	// Method on view
	assert.Equal(t, "origin/feature/bar (remota)", view.FormatBranch(remoteBranch, i18n.Es))
}

func TestPriorityOptionsAndLabels(t *testing.T) {
	options := state.PriorityOptions()
	assert.Len(t, options, 4)

	assert.Equal(t, "low", options[0].Value)
	assert.Equal(t, "normal", options[1].Value)
	assert.Equal(t, "high", options[2].Value)
	assert.Equal(t, "urgent", options[3].Value)

	assert.Equal(t, "Baja", state.PriorityLabel("low", i18n.Es))
	assert.Equal(t, "Normal", state.PriorityLabel("normal", i18n.Es))
	assert.Equal(t, "Alta", state.PriorityLabel("high", i18n.Es))
	assert.Equal(t, "Urgente", state.PriorityLabel("urgent", i18n.Es))

	assert.Equal(t, "Low", state.PriorityLabel("low", i18n.En))
	assert.Equal(t, "Normal", state.PriorityLabel("normal", i18n.En))
	assert.Equal(t, "High", state.PriorityLabel("high", i18n.En))
	assert.Equal(t, "Urgent", state.PriorityLabel("urgent", i18n.En))

	assert.Equal(t, "Baja", options[0].LabelText(i18n.Es))
	assert.Equal(t, "Low", options[0].LabelText(i18n.En))

	// Triangulation: case insensitivity and fallback for unknown
	assert.Equal(t, "Baja", state.PriorityLabel("LOW", i18n.Es))
	assert.Equal(t, "Urgente", state.PriorityLabel("Urgent", i18n.Es))
	assert.Equal(t, "Normal", state.PriorityLabel("unknown", i18n.Es))
	assert.Equal(t, "Normal", state.PriorityLabel("", i18n.Es))

}
