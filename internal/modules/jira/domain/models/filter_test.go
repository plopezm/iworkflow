package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/jira/domain/models"
)

func TestEmptyFilterMatchesEverything(t *testing.T) {
	assert.True(t, models.IssueFilter{}.Matches(sampleIssue("A-1")))
}

func TestSearchIsCaseInsensitiveOverKeySummaryAssigneeAndLabels(t *testing.T) {
	issue := sampleIssue("ABC-7")
	for _, term := range []string{"abc-7", "RESUMEN", "ana", "BACKEND"} {
		assert.True(t, models.IssueFilter{Search: term}.Matches(issue), term)
	}
	assert.False(t, models.IssueFilter{Search: "zzz"}.Matches(issue))
}

func TestFiltersByStatusAndLocalState(t *testing.T) {
	issue := sampleIssue("A-1")
	issue.LocalState = models.IssueLocalStateIgnored
	done := "Done"
	ignored := models.IssueLocalStateIgnored
	todo := "To Do"

	assert.False(t, models.IssueFilter{Status: &done}.Matches(issue))
	assert.True(t, models.IssueFilter{LocalState: &ignored, Status: &todo}.Matches(issue))
}

func TestApplyKeepsOrderOfTheMatches(t *testing.T) {
	ignored := sampleIssue("A-2")
	ignored.LocalState = models.IssueLocalStateIgnored
	state := models.IssueLocalStateNew

	kept := models.IssueFilter{LocalState: &state}.Apply([]models.Issue{sampleIssue("A-1"), ignored, sampleIssue("A-3")})

	assert.Len(t, kept, 2)
	assert.Equal(t, "A-1", kept[0].Key())
	assert.Equal(t, "A-3", kept[1].Key())
}
