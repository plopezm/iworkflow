package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// fakeTaskInfo answers the task snapshot of a proposal, or fails.
type fakeTaskInfo struct {
	snapshot outbound.TaskSnapshot
	err      error
}

var _ outbound.TaskInfo = (*fakeTaskInfo)(nil)

func (f *fakeTaskInfo) Info(_ context.Context, _ string) (outbound.TaskSnapshot, error) {
	return f.snapshot, f.err
}

func TestGetWorklogProposalProposesTheUnloggedAgentTime(t *testing.T) {
	tests := []struct {
		name     string
		runTime  time.Duration
		logged   []int
		wantTime time.Duration
	}{
		{"nothing logged yet rounds the agent time to five minutes", 82 * time.Minute, nil, 80 * time.Minute},
		{"a short run still proposes the five-minute minimum", 2 * time.Minute, nil, 5 * time.Minute},
		{"time already logged is not proposed again", 90 * time.Minute, []int{3600}, 30 * time.Minute},
		{"everything logged proposes nothing", 60 * time.Minute, []int{3600}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worklogs := &fakeWorklogs{}
			for index, seconds := range tt.logged {
				require.NoError(t, worklogs.Add(context.Background(), models.WorklogRecord{
					TaskID: "task-1", JiraID: string(rune('0' + index)), Seconds: seconds, Comment: "previo",
				}))
			}
			taskInfo := &fakeTaskInfo{snapshot: outbound.TaskSnapshot{Title: "Migrar login", AgentRunTime: tt.runTime}}
			useCase := application.NewGetWorklogProposal(taskInfo, application.NewListTaskWorklogs(worklogs))

			got, err := useCase.Execute(context.Background(), "task-1")

			require.NoError(t, err)
			assert.Equal(t, tt.wantTime, got.Time)
		})
	}
}

func TestGetWorklogProposalCarriesFinishedLoggedAndTheDescription(t *testing.T) {
	worklogs := &fakeWorklogs{}
	require.NoError(t, worklogs.Add(context.Background(), models.WorklogRecord{TaskID: "task-1", Seconds: 600, Comment: "x"}))
	taskInfo := &fakeTaskInfo{snapshot: outbound.TaskSnapshot{
		Title:        "Migrar login",
		Finished:     true,
		AgentRunTime: 30 * time.Minute,
		Commits:      []string{"abc1234 add login", "def5678 fix tests"},
	}}
	useCase := application.NewGetWorklogProposal(taskInfo, application.NewListTaskWorklogs(worklogs))

	got, err := useCase.Execute(context.Background(), "task-1")

	require.NoError(t, err)
	assert.True(t, got.Finished)
	assert.Equal(t, 10*time.Minute, got.Logged)
	assert.Equal(t, 20*time.Minute, got.Time)
	assert.Equal(t, "Migrar login\n\n- add login\n- fix tests", got.Description)
}

func TestGetWorklogProposalFailsWhenTheTaskCannotBeRead(t *testing.T) {
	useCase := application.NewGetWorklogProposal(
		&fakeTaskInfo{err: errors.New("tarea no encontrada")},
		application.NewListTaskWorklogs(&fakeWorklogs{}),
	)

	_, err := useCase.Execute(context.Background(), "task-1")

	require.EqualError(t, err, "tarea no encontrada")
}
