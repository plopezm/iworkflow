package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/jira/domain/models"
)

func TestIsValidIssueKeyAcceptsRegularKeys(t *testing.T) {
	assert.True(t, models.IsValidIssueKey("PROJ-1"))
	assert.True(t, models.IsValidIssueKey("AB_C2-1234"))
}

func TestIsValidIssueKeyRejectsKeysThatCouldAlterTheRequestPath(t *testing.T) {
	for _, key := range []string{"", "PROJ", "PROJ-", "-1", "1A-2", "PROJ-1/comment", "../rest", "A-1?x=1", "A-1 "} {
		assert.False(t, models.IsValidIssueKey(key), "%q should be rejected", key)
	}
}

func TestIssueKeyComesFromTheRemoteData(t *testing.T) {
	assert.Equal(t, "ABC-7", sampleIssue("ABC-7").Key())
}
