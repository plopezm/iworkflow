package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/domain/models"
)

func epicTestIssue(key, kind, status string, epic *string) models.Issue {
	return models.Issue{Remote: models.RemoteIssue{Key: key, IssueType: kind, Status: status, EpicKey: epic}}
}

func epicPtr(value string) *string { return &value }

func TestGroupByEpicGroupsChildrenUnderTheirEpic(t *testing.T) {
	groups := models.GroupByEpic([]models.Issue{
		epicTestIssue("A-2", "Story", "Done", epicPtr("A-1")),
		epicTestIssue("A-1", "Epic", "In Progress", nil),
		epicTestIssue("A-3", "Bug", "To Do", epicPtr("A-1")),
		epicTestIssue("B-1", "Task", "To Do", epicPtr("X-9")),
		epicTestIssue("C-1", "Task", "To Do", nil),
	})

	require.Len(t, groups, 3)
	assert.Equal(t, "A-1", groups[0].Key)
	require.NotNil(t, groups[0].Epic)
	assert.Len(t, groups[0].Children, 2)
	assert.Equal(t, 1, groups[0].Done())
	assert.Equal(t, "X-9", groups[1].Key)
	assert.Nil(t, groups[1].Epic, "an epic that was not imported keeps only its key")
	assert.Equal(t, models.NoEpicKey, groups[2].Key)
	assert.Len(t, groups[2].Children, 1)
}

func TestGroupByEpicKeepsEmptyEpicsAndOmitsTheEmptyNoEpicGroup(t *testing.T) {
	groups := models.GroupByEpic([]models.Issue{epicTestIssue("A-1", "epic", "To Do", nil)})

	require.Len(t, groups, 1)
	assert.Empty(t, groups[0].Children)
}

func TestStatusCountsFollowFirstAppearance(t *testing.T) {
	group := models.EpicGroup{Children: []models.Issue{
		epicTestIssue("A-1", "Task", "To Do", nil),
		epicTestIssue("A-2", "Task", "Done", nil),
		epicTestIssue("A-3", "Task", "To Do", nil),
	}}

	assert.Equal(t, []models.StatusCount{{Status: "To Do", Count: 2}, {Status: "Done", Count: 1}}, group.StatusCounts())
}

func TestIsDoneStatus(t *testing.T) {
	assert.True(t, models.IsDoneStatus(" Hecho "))
	assert.True(t, models.IsDoneStatus("CLOSED"))
	assert.False(t, models.IsDoneStatus("In Progress"))
}
