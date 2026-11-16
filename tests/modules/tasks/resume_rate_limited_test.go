package tasks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// sweepRuntime is the agent runtime of the sweep test: the start works, the send fails, and the stops are recorded.
// The methods the sweep does not use panic through the embedded nil interface.
type sweepRuntime struct {
	outbound.AgentRuntime

	sendErr error
	stopped []models.TaskID
}

func (*sweepRuntime) IsRunning(context.Context, models.TaskID) bool { return false }

func (*sweepRuntime) Start(context.Context, models.TaskID, string, bool, models.LaunchOptions, inbound.AgentEventSink) error {
	return nil
}

func (r *sweepRuntime) Send(context.Context, models.TaskID, models.AgentCommand) error {
	return r.sendErr
}

func (r *sweepRuntime) Stop(_ context.Context, id models.TaskID) error {
	r.stopped = append(r.stopped, id)
	return nil
}

func (*sweepRuntime) RuntimeCapabilities(*models.RuntimeID) models.AgentCapabilities {
	return models.FullAgentCapabilities()
}

type sweepWorktree struct{ outbound.Worktree }

func (sweepWorktree) Exists(context.Context, string) bool { return true }

type sweepSettings struct{}

func (sweepSettings) Prepare(context.Context, string, models.TaskAgentOptions) (models.LaunchOptions, error) {
	return models.LaunchOptions{}, nil
}

type sweepSink struct{}

func (sweepSink) Handle(context.Context, models.TaskID, models.AgentEvent) {}

type sweepPublisher struct{}

func (sweepPublisher) Publish(models.TaskEvent) {}

type sweepProbe struct{}

func (sweepProbe) HasCodegraph(context.Context, string) bool { return false }
func (sweepProbe) EngramAvailable(context.Context) bool      { return false }

type sweepWorkspaces struct{ ref models.WorkspaceRef }

func (w sweepWorkspaces) Find(_ context.Context, id string) (*models.WorkspaceRef, error) {
	if id != w.ref.ID {
		return nil, nil //nolint:nilnil // the port reports a missing workspace as nil without error
	}
	found := w.ref
	return &found, nil
}

func (w sweepWorkspaces) List(context.Context) ([]models.WorkspaceRef, error) {
	return []models.WorkspaceRef{w.ref}, nil
}

// A rate-limited task whose resume works but whose continuation fails goes back to rate_limited with a new schedule
// on the real SQLite repository: the status write must not erase the schedule the sweep stores afterwards.
func TestResumeRateLimitedFailedContinuationParksTheTaskAgainInSQLite(t *testing.T) {
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/demo")
	task := newTask(t, workspaceID, "limite", models.StatusRateLimited)
	due := fixedClock.Add(-time.Minute).Truncate(time.Microsecond)
	task.UsageLimit = models.UsageLimitState{ResumeAt: &due, Attempts: 0}
	insertTask(t, repo, task)

	workspaces := sweepWorkspaces{ref: models.WorkspaceRef{ID: workspaceID, Name: "demo", Path: "/repos/demo", BaseBranch: "main"}}
	clock := application.Clock(repo.clock.Now)
	lifecycle := application.NewTaskLifecycle(repo.Tasks, sweepPublisher{}, workspaces, clock)
	runtime := &sweepRuntime{sendErr: errors.New("send failed")}
	related := application.NewPrepareRelatedLaunch(application.NewListTaskRelatedProjects(repo.RelatedProjects, workspaces), sweepProbe{})
	resume := application.NewResumeTask(lifecycle, sweepWorktree{}, runtime, sweepSettings{}, sweepSink{}, related)
	sweep := application.NewResumeRateLimitedTasks(lifecycle, runtime, resume, application.NewSendMessage(lifecycle, runtime))

	now := fixedClock
	resumed, err := sweep.Execute(context.Background(), now)

	require.NoError(t, err)
	assert.Equal(t, 0, resumed)
	stored := found(t, repo, task.ID)
	assert.Equal(t, models.StatusRateLimited, stored.Status)
	require.NotNil(t, stored.UsageLimit.ResumeAt, "the new schedule must survive the status write")
	assert.True(t, stored.UsageLimit.ResumeAt.Equal(now.Add(5*time.Minute)), "got %s", stored.UsageLimit.ResumeAt)
	assert.Equal(t, uint32(1), stored.UsageLimit.Attempts)
	assert.Equal(t, []models.TaskID{task.ID}, runtime.stopped)
}
