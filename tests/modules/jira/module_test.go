package jira_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/di"
)

// stubTasks records the conversions and answers with a fixed task id.
type stubTasks struct {
	created []outbound.NewLinkedTask
}

func (s *stubTasks) Create(_ context.Context, task outbound.NewLinkedTask) (outbound.CreatedLinkedTask, error) {
	s.created = append(s.created, task)
	return outbound.CreatedLinkedTask{TaskID: "task-42"}, nil
}

// stubWorkspaces offers one workspace with one branch.
type stubWorkspaces struct{}

func (stubWorkspaces) Workspaces(context.Context) ([]models.WorkspaceOption, error) {
	return []models.WorkspaceOption{{ID: "w1", Name: "app", BaseBranch: "main"}}, nil
}

func (stubWorkspaces) Branches(context.Context, string) ([]models.BranchOption, error) {
	return []models.BranchOption{{Name: "main"}}, nil
}

// TestModuleConnectSyncConvertAndDisconnect runs the whole flow through the composition root against a
// migrated database and the fake Jira server.
func TestModuleConnectSyncConvertAndDisconnect(t *testing.T) {
	ctx := context.Background()
	fake := newFakeJira(t)
	tasks := &stubTasks{}
	module := di.NewModule(di.Deps{DB: dbtest.New(t), Tasks: tasks, Workspaces: stubWorkspaces{}})

	saved, err := module.SaveConnection.Execute(ctx, application.SaveConnectionInput{BaseURL: fake.url(), Token: testToken})
	require.NoError(t, err)
	assert.Equal(t, fake.url(), saved.BaseURL())

	outcome, err := module.SyncIssues.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, application.SyncOutcome{Total: 3, Added: 3}, application.SyncOutcome{Total: outcome.Total, Added: outcome.Added})

	listing, err := module.ListIssues.Execute(ctx, models.IssueFilter{})
	require.NoError(t, err)
	require.Len(t, listing.Lanes, 1)
	assert.True(t, listing.Lanes[0].Lane.IsDefault)
	assert.Len(t, listing.Lanes[0].Issues, 3)
	assert.Equal(t, []string{"To Do"}, listing.Statuses)

	converted, err := module.ConvertIssue.Execute(ctx, models.ConversionRequest{
		Key:         "A-1",
		WorkspaceID: "w1",
		Title:       "Título",
		Prompt:      "Prompt",
		Priority:    models.PriorityHintHigh,
	})
	require.NoError(t, err)
	assert.Equal(t, "task-42", converted.TaskID)
	require.Len(t, tasks.created, 1)

	issue, err := module.GetIssue.Execute(ctx, "A-1")
	require.NoError(t, err)
	assert.Equal(t, models.IssueLocalStateConverted, issue.LocalState)
	require.NotNil(t, issue.TaskID)
	assert.Equal(t, "task-42", *issue.TaskID)

	_, err = module.ConvertIssue.Execute(ctx, models.ConversionRequest{Key: "A-1", WorkspaceID: "w1", Title: "t", Prompt: "p"})
	require.ErrorIs(t, err, jiraerrors.AlreadyConverted(""))

	require.NoError(t, module.DeleteConnection.Execute(ctx))
	stored, err := module.GetConnection.Execute(ctx)
	require.NoError(t, err)
	assert.Nil(t, stored)
	_, err = module.SyncIssues.Execute(ctx)
	require.ErrorIs(t, err, jiraerrors.NotConfigured())
}

// TestModuleRefusesToSaveAWrongTokenAndStoresNothing covers the verification gate of the connection.
func TestModuleRefusesToSaveAWrongTokenAndStoresNothing(t *testing.T) {
	ctx := context.Background()
	fake := newFakeJira(t)
	module := di.NewModule(di.Deps{DB: dbtest.New(t), Tasks: &stubTasks{}, Workspaces: stubWorkspaces{}})

	_, err := module.SaveConnection.Execute(ctx, application.SaveConnectionInput{BaseURL: fake.url(), Token: "wrong"})

	require.ErrorIs(t, err, jiraerrors.Unauthorized())
	stored, err := module.GetConnection.Execute(ctx)
	require.NoError(t, err)
	assert.Nil(t, stored)
}
