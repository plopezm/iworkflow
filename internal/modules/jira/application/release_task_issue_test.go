package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/domain/models"
)

func TestReleaseTaskIssueMakesTheLinkedIssueNewAgain(t *testing.T) {
	taskID := "task-1"
	linked := issueIn("A-1", models.IssueLocalStateConverted)
	linked.TaskID = &taskID
	issues := &fakeIssues{issues: []models.Issue{linked}}

	require.NoError(t, application.NewReleaseTaskIssue(issues).Execute(context.Background(), taskID))

	require.Len(t, issues.setStates, 1)
	assert.Equal(t, "A-1", issues.setStates[0].key)
	assert.Equal(t, models.IssueLocalStateNew, issues.setStates[0].state)
	assert.Nil(t, issues.setStates[0].taskID, "the task link is removed")
}

func TestReleaseTaskIssueDoesNothingWhenTheTaskHasNoIssue(t *testing.T) {
	issues := &fakeIssues{issues: []models.Issue{issueIn("A-1", models.IssueLocalStateNew)}}

	require.NoError(t, application.NewReleaseTaskIssue(issues).Execute(context.Background(), "task-1"))

	assert.Empty(t, issues.setStates)
}

func TestReleaseTaskIssueReportsALookupFailure(t *testing.T) {
	issues := &fakeIssues{findErr: errors.New("db down")}

	err := application.NewReleaseTaskIssue(issues).Execute(context.Background(), "task-1")

	require.EqualError(t, err, "db down")
	assert.Empty(t, issues.setStates)
}

func TestReleaseTaskIssueLeavesAnIssueThatIsNoLongerConverted(t *testing.T) {
	taskID := "task-1"
	ignored := issueIn("A-1", models.IssueLocalStateIgnored)
	ignored.TaskID = &taskID
	issues := &fakeIssues{issues: []models.Issue{ignored}}

	require.NoError(t, application.NewReleaseTaskIssue(issues).Execute(context.Background(), taskID))

	assert.Empty(t, issues.setStates)
}
