package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/jira/domain/models"
)

func TestIssueLocalStateRoundTripsEveryState(t *testing.T) {
	for _, state := range []models.IssueLocalState{
		models.IssueLocalStateNew,
		models.IssueLocalStateConverted,
		models.IssueLocalStateIgnored,
	} {
		assert.Equal(t, state, models.ParseIssueLocalState(state.String()))
	}
}

func TestUnknownIssueLocalStateFallsBackToNew(t *testing.T) {
	assert.Equal(t, models.IssueLocalStateNew, models.ParseIssueLocalState("???"))
}

func TestPriorityHintMapsDefaultJiraPriorities(t *testing.T) {
	cases := map[string]models.PriorityHint{
		"Blocker":  models.PriorityHintUrgent,
		"Critical": models.PriorityHintUrgent,
		"Highest":  models.PriorityHintUrgent,
		"High":     models.PriorityHintHigh,
		"Major":    models.PriorityHintHigh,
		"Medium":   models.PriorityHintNormal,
		"Minor":    models.PriorityHintLow,
		"Lowest":   models.PriorityHintLow,
		"Trivial":  models.PriorityHintLow,
	}
	for name, expected := range cases {
		assert.Equal(t, expected, models.PriorityHintFromJira(name), name)
	}
}

func TestMissingOrUnknownPriorityIsNormal(t *testing.T) {
	assert.Equal(t, models.PriorityHintNormal, models.PriorityHintFromJira(""))
	assert.Equal(t, models.PriorityHintNormal, models.PriorityHintFromJira("Whatever"))
}

func TestPriorityHintParseRoundTrips(t *testing.T) {
	for _, priority := range []models.PriorityHint{
		models.PriorityHintLow,
		models.PriorityHintNormal,
		models.PriorityHintHigh,
		models.PriorityHintUrgent,
	} {
		assert.Equal(t, priority, models.ParsePriorityHint(priority.String()))
	}
	assert.Equal(t, models.PriorityHintNormal, models.ParsePriorityHint("bogus"))
}
