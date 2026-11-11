package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/domain/models"
)

func epicFixture(key, kind, status string, epic string) models.Issue {
	issue := models.Issue{Remote: models.RemoteIssue{Key: key, IssueType: kind, Status: status}}
	if epic != "" {
		issue.Remote.EpicKey = &epic
	}
	return issue
}

func TestListEpicsGroupsAndReportsStatuses(t *testing.T) {
	repo := &fakeIssues{issues: []models.Issue{
		epicFixture("A-1", "Epic", "To Do", ""),
		epicFixture("A-2", "Story", "Done", "A-1"),
		epicFixture("B-1", "Task", "In Progress", ""),
	}}

	listing, err := application.NewListEpics(repo).Execute(context.Background(), models.IssueFilter{})

	require.NoError(t, err)
	require.Len(t, listing.Groups, 2)
	assert.Equal(t, "A-1", listing.Groups[0].Key)
	assert.Equal(t, models.NoEpicKey, listing.Groups[1].Key)
	assert.Equal(t, []string{"Done", "In Progress", "To Do"}, listing.Statuses)
	assert.Equal(t, 3, listing.Total)
}

func TestListEpicsHidesGroupsWithoutMatchesWhileFiltering(t *testing.T) {
	repo := &fakeIssues{issues: []models.Issue{
		epicFixture("A-1", "Epic", "To Do", ""),
		epicFixture("A-2", "Story", "Done", "A-1"),
		epicFixture("C-1", "Epic", "To Do", ""),
		epicFixture("C-2", "Story", "To Do", "C-1"),
	}}
	done := "Done"

	listing, err := application.NewListEpics(repo).Execute(context.Background(), models.IssueFilter{Status: &done})

	require.NoError(t, err)
	require.Len(t, listing.Groups, 1)
	assert.Equal(t, "A-1", listing.Groups[0].Key)
	assert.Equal(t, 4, listing.Total, "the total ignores the filter")
}
