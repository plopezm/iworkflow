package dto_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

func TestWorklogsFromDomainFormatsTimeInJiraFormatInOrder(t *testing.T) {
	t.Parallel()
	logged := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	records := []models.WorklogRecord{
		{JiraID: "10", Seconds: 5400, Comment: "uno", LoggedAt: logged},
		{JiraID: "11", Seconds: 900, Comment: "dos", LoggedAt: logged},
	}

	got := dto.WorklogsFromDomain(records)

	if assert.Len(t, got, 2) {
		assert.Equal(t, "10", got[0].JiraID)
		assert.Equal(t, "1h 30m", got[0].Time)
		assert.Equal(t, "uno", got[0].Comment)
		assert.Equal(t, "15m", got[1].Time)
	}
}

func TestWorklogsFromDomainIsEmptyWithoutRecords(t *testing.T) {
	t.Parallel()
	assert.Empty(t, dto.WorklogsFromDomain(nil))
}
