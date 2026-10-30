package models_test

import (
	"time"

	"iworkflow/internal/modules/jira/domain/models"
)

// sampleRemote is the Jira data of an issue, as the tests of the module use it.
func sampleRemote(key string) models.RemoteIssue {
	priority := "High"
	assignee := "Ana"
	reporter := "Luis"
	return models.RemoteIssue{
		Key:           key,
		Summary:       "Resumen de " + key,
		Description:   "Descripción larga",
		Status:        "To Do",
		IssueType:     "Bug",
		Priority:      &priority,
		Assignee:      &assignee,
		Reporter:      &reporter,
		Labels:        []string{"backend"},
		URL:           "https://jira.local/browse/" + key,
		JiraUpdatedAt: time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC),
	}
}

// sampleIssue is an imported, not yet converted issue.
func sampleIssue(key string) models.Issue {
	return models.Issue{
		Remote:     sampleRemote(key),
		LocalState: models.IssueLocalStateNew,
		SyncedAt:   time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC),
	}
}
