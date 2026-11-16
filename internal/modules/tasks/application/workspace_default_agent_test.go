package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// workspaceWithDefault returns a lookup over the workspace ws-1 whose default agent is runtime (nil for none).
func workspaceWithDefault(runtime *string) knownWorkspaces {
	return knownWorkspaces{
		ownWorkspace: {ID: ownWorkspace, Name: "demo", Path: "/repos/demo", BaseBranch: "main", DefaultRuntime: runtime},
	}
}

// Spec workspace-default-agent S2/S3: EffectiveAgent resolves the workspace default only when the task sets no
// runtime, and any unusable default (missing workspace, failed lookup, invalid identifier) leaves the options alone.
func TestEffectiveAgentResolvesTheWorkspaceDefaultRuntime(t *testing.T) {
	t.Parallel()
	claude := "claude"
	invalid := "Not A Runtime!"
	pi, err := models.ParseRuntimeID("pi")
	require.NoError(t, err)
	claudeID, err := models.ParseRuntimeID("claude")
	require.NoError(t, err)
	lookupFailure := werrors.Repository("workspaces no disponibles")

	cases := []struct {
		name      string
		workspace outbound.WorkspaceLookup
		agent     models.TaskAgentOptions
		want      models.TaskAgentOptions
	}{
		{
			name:      "task without runtime takes the workspace default",
			workspace: workspaceWithDefault(&claude),
			agent:     models.TaskAgentOptions{},
			want:      models.TaskAgentOptions{Runtime: &claudeID},
		},
		{
			name:      "explicit task runtime wins over the workspace default",
			workspace: workspaceWithDefault(&claude),
			agent:     models.TaskAgentOptions{Runtime: &pi},
			want:      models.TaskAgentOptions{Runtime: &pi},
		},
		{
			name:      "workspace without default leaves the options unchanged",
			workspace: workspaceWithDefault(nil),
			agent:     models.TaskAgentOptions{},
			want:      models.TaskAgentOptions{},
		},
		{
			name:      "unknown workspace leaves the options unchanged",
			workspace: knownWorkspaces{},
			agent:     models.TaskAgentOptions{},
			want:      models.TaskAgentOptions{},
		},
		{
			name:      "workspace lookup failure leaves the options unchanged",
			workspace: failingWorkspaces{err: lookupFailure},
			agent:     models.TaskAgentOptions{},
			want:      models.TaskAgentOptions{},
		},
		{
			name:      "invalid default runtime identifier leaves the options unchanged",
			workspace: workspaceWithDefault(&invalid),
			agent:     models.TaskAgentOptions{},
			want:      models.TaskAgentOptions{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusBacklog)
			task.Agent = tc.agent
			lifecycle := application.NewTaskLifecycle(newFakeRepository(task), &fakePublisher{}, tc.workspace, clock)

			got := lifecycle.EffectiveAgent(context.Background(), task)

			assert.Equal(t, tc.want, got)
		})
	}
}

// Spec workspace-default-agent S2: a launch hands the workspace default to LaunchSettings.Prepare as the runtime.
func TestStartTaskPassesTheWorkspaceDefaultRuntimeToLaunchSettings(t *testing.T) {
	t.Parallel()
	claude := "claude"
	claudeID, err := models.ParseRuntimeID("claude")
	require.NoError(t, err)
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	start := startWithWorkspaces(fx, workspaceWithDefault(&claude))

	started, err := start.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	require.Len(t, fx.settings.agents, 1)
	assert.Equal(t, models.TaskAgentOptions{Runtime: &claudeID}, fx.settings.agents[0])
}

// Spec workspace-default-agent S2: a task with no workspace default keeps its own agent options untouched.
func TestStartTaskWithoutWorkspaceDefaultPassesTheTaskAgentOptionsUnchanged(t *testing.T) {
	t.Parallel()
	model := "opus"
	thinking := "high"
	task := newTask(models.StatusBacklog)
	task.Agent = models.TaskAgentOptions{ModelID: &model, Thinking: &thinking}
	fx := newStartFixture(task)
	start := startWithWorkspaces(fx, workspaceWithDefault(nil))

	started, err := start.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	require.Len(t, fx.settings.agents, 1)
	assert.Equal(t, task.Agent, fx.settings.agents[0])
}

// startWithWorkspaces builds StartTask over the fixture's fakes, with the given workspace lookup.
func startWithWorkspaces(fx *startFixture, workspaces outbound.WorkspaceLookup) *application.StartTask {
	lifecycle := application.NewTaskLifecycle(fx.repo, fx.publisher, workspaces, clock)
	related := application.NewPrepareRelatedLaunch(application.NewListTaskRelatedProjects(fx.related, relatedWorkspaces()), fx.probe)
	return application.NewStartTask(lifecycle, fx.worktrees, fx.runtime, fx.settings, &resumeSink{}, fx.attachments, fx.storage, related, newUUID, clock)
}
