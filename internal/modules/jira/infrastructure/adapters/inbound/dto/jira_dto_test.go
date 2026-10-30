package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

func TestConnectionDTONeverCarriesTheToken(t *testing.T) {
	connection, err := models.NewConnection("https://j", "super-secret", true)
	require.NoError(t, err)

	encoded, err := json.Marshal(dto.ConnectionFromDomain(connection))
	require.NoError(t, err)

	assert.NotContains(t, string(encoded), "super-secret")
	assert.Contains(t, string(encoded), `"accept_invalid_certs":true`)
}

func TestSaveConnectionDTOMapsTheFormToTheUseCaseInput(t *testing.T) {
	input := dto.SaveConnectionDTO{BaseURL: "https://j", Token: "", AcceptInvalidCerts: true}.ToSaveInput()

	assert.Equal(t, "https://j", input.BaseURL)
	assert.Empty(t, input.Token, "a blank token keeps the stored one")
	assert.True(t, input.AcceptInvalidCerts)
}

func TestFilterDTODropsABlankStatusAndKeepsAKnownState(t *testing.T) {
	blank := "  "
	ignored := "ignored"

	filter := dto.IssueFilterDTO{Status: &blank, LocalState: &ignored}.ToDomain()

	assert.Nil(t, filter.Status)
	require.NotNil(t, filter.LocalState)
	assert.Equal(t, models.IssueLocalStateIgnored, *filter.LocalState)
}

func TestFilterDTOIgnoresAnUnknownState(t *testing.T) {
	unknown := "bogus"

	filter := dto.IssueFilterDTO{LocalState: &unknown}.ToDomain()

	assert.Nil(t, filter.LocalState)
}

func TestConvertDTOFallsBackToNormalPriority(t *testing.T) {
	request := dto.ConvertIssueDTO{Key: "A-1", Priority: "???"}.ToConversionRequest()

	assert.Equal(t, models.PriorityHintNormal, request.Priority)
	assert.Equal(t, "A-1", request.Key)
}

func TestIssueDTOKeepsEmptyLabelsAsAnArray(t *testing.T) {
	issue := models.Issue{Remote: models.RemoteIssue{Key: "A-1"}, LocalState: models.IssueLocalStateNew}

	encoded, err := json.Marshal(dto.IssueFromDomain(issue))
	require.NoError(t, err)

	assert.Contains(t, string(encoded), `"labels":[]`)
}
