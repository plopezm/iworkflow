package application_test

import (
	"bytes"
	"context"
	"log/slog"
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

// Spec workspace-default-agent T4: the first launch pins the workspace default into the task, so the task keeps the
// agent it started with when the default changes later.
func TestStartTaskPinsTheWorkspaceDefaultIntoTheTask(t *testing.T) {
	t.Parallel()
	claude := "claude"
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	start := startWithWorkspaces(fx, workspaceWithDefault(&claude))

	started, err := start.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	assert.Equal(t, []string{"claude"}, fx.repo.pinned)
	stored := fx.repo.tasks[task.ID]
	require.NotNil(t, stored.Agent.Runtime)
	assert.Equal(t, "claude", stored.Agent.Runtime.String())
}

// Spec workspace-default-agent T4: a task that sets its own runtime is never pinned, and keeps that runtime.
func TestStartTaskDoesNotPinATaskWithItsOwnRuntime(t *testing.T) {
	t.Parallel()
	claude := "claude"
	task := newTask(models.StatusBacklog)
	task.Agent = models.TaskAgentOptions{Runtime: runtimeID(t, "pi")}
	fx := newStartFixture(task)
	start := startWithWorkspaces(fx, workspaceWithDefault(&claude))

	started, err := start.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	assert.Empty(t, fx.repo.pinned)
	require.Len(t, fx.settings.agents, 1)
	assert.Equal(t, models.TaskAgentOptions{Runtime: runtimeID(t, "pi")}, fx.settings.agents[0])
	require.NotNil(t, fx.repo.tasks[task.ID].Agent.Runtime)
	assert.Equal(t, "pi", fx.repo.tasks[task.ID].Agent.Runtime.String())
}

// Spec workspace-default-agent T4: without a workspace default there is nothing to pin.
func TestStartTaskWithoutWorkspaceDefaultPinsNothing(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	start := startWithWorkspaces(fx, workspaceWithDefault(nil))

	started, err := start.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	assert.Empty(t, fx.repo.pinned)
	assert.Nil(t, fx.repo.tasks[task.ID].Agent.Runtime)
}

// Spec workspace-default-agent T4: a failed pin only loses the stored copy; the launch still uses the default.
func TestStartTaskFailedPinStillLaunchesWithTheWorkspaceDefault(t *testing.T) {
	t.Parallel()
	claude := "claude"
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.repo.pinErr = werrors.Repository("base de datos ocupada")
	start := startWithWorkspaces(fx, workspaceWithDefault(&claude))

	started, err := start.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	assert.Empty(t, fx.repo.pinned)
	assert.Nil(t, fx.repo.tasks[task.ID].Agent.Runtime)
	require.Len(t, fx.settings.agents, 1)
	require.NotNil(t, fx.settings.agents[0].Runtime)
	assert.Equal(t, "claude", fx.settings.agents[0].Runtime.String())
}

// pinnedByStart launches a task while the workspace default is claude, then leaves it interrupted and changes the
// default to pi. It returns the fixture, the shared lookup (already changed) and the task.
func pinnedByStart(t *testing.T) (*startFixture, knownWorkspaces, models.Task) {
	t.Helper()
	claude := "claude"
	pi := "pi"
	workspaces := workspaceWithDefault(&claude)
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)

	_, err := startWithWorkspaces(fx, workspaces).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Equal(t, []string{"claude"}, fx.repo.pinned)
	stored := fx.repo.tasks[task.ID]
	stored.Status = models.StatusInterrupted
	fx.repo.tasks[task.ID] = stored
	workspaces[ownWorkspace] = models.WorkspaceRef{ID: ownWorkspace, Name: "demo", Path: "/repos/demo", BaseBranch: "main", DefaultRuntime: &pi}
	return fx, workspaces, task
}

// Spec workspace-default-agent T4: resuming a task keeps the runtime it was launched with after the default changed.
func TestResumeUsesTheRuntimePinnedBeforeTheWorkspaceDefaultChanged(t *testing.T) {
	t.Parallel()
	fx, workspaces, task := pinnedByStart(t)
	lifecycle := application.NewTaskLifecycle(fx.repo, fx.publisher, workspaces, clock)
	resume := application.NewResumeTask(lifecycle, fx.worktrees, fx.runtime, fx.settings, &resumeSink{}, relatedLaunchOf(fx))

	err := resume.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, fx.settings.agents, 2)
	require.NotNil(t, fx.settings.agents[1].Runtime)
	assert.Equal(t, "claude", fx.settings.agents[1].Runtime.String())
}

// Spec workspace-default-agent T4: a new session keeps the pinned runtime after the workspace default changed.
func TestNewSessionUsesTheRuntimePinnedBeforeTheWorkspaceDefaultChanged(t *testing.T) {
	t.Parallel()
	fx, workspaces, task := pinnedByStart(t)
	lifecycle := application.NewTaskLifecycle(fx.repo, fx.publisher, workspaces, clock)
	newSession := application.NewNewSession(lifecycle, fx.worktrees, fx.runtime, fx.settings, &resumeSink{}, relatedLaunchOf(fx), flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())

	err := newSession.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, fx.settings.agents, 2)
	require.NotNil(t, fx.settings.agents[1].Runtime)
	assert.Equal(t, "claude", fx.settings.agents[1].Runtime.String())
}

// relatedLaunchOf builds the related-projects step over the fixture's fakes.
func relatedLaunchOf(fx *startFixture) *application.PrepareRelatedLaunch {
	return application.NewPrepareRelatedLaunch(application.NewListTaskRelatedProjects(fx.related, relatedWorkspaces()), fx.probe)
}

// Spec workspace-default-agent T4: EffectiveAgent logs a warning when the default cannot be used, and stays silent
// when the workspace has no default. It is not parallel: it replaces the process-wide default logger.
func TestEffectiveAgentLogsWhyTheWorkspaceDefaultIsNotUsed(t *testing.T) {
	invalid := "Not A Runtime!"
	cases := []struct {
		name      string
		workspace outbound.WorkspaceLookup
		wantLog   string
	}{
		{name: "failed workspace lookup warns", workspace: failingWorkspaces{err: werrors.Repository("workspaces no disponibles")}, wantLog: "could not read the workspace"},
		{name: "invalid default warns", workspace: workspaceWithDefault(&invalid), wantLog: "not valid"},
		{name: "nil default logs nothing", workspace: workspaceWithDefault(nil), wantLog: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			task := newTask(models.StatusBacklog)
			lifecycle := application.NewTaskLifecycle(newFakeRepository(task), &fakePublisher{}, tc.workspace, clock)

			got := lifecycle.EffectiveAgent(context.Background(), task)

			assert.Equal(t, models.TaskAgentOptions{}, got)
			if tc.wantLog == "" {
				assert.Empty(t, logs.String())
				return
			}
			assert.Contains(t, logs.String(), "level=WARN")
			assert.Contains(t, logs.String(), tc.wantLog)
			assert.Contains(t, logs.String(), "task_id="+task.ID.String())
		})
	}
}

// Review finding: the default is stored only once the agent started with it, so a launch that fails pins nothing and
// a later change of the default still applies to the failed task.
func TestStartTaskThatFailsToLaunchPinsNothing(t *testing.T) {
	t.Parallel()
	claude := "claude"
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.runtime.startErrs[task.ID] = werrors.Runtime("no existe claude")
	start := startWithWorkspaces(fx, workspaceWithDefault(&claude))

	_, err := start.Execute(context.Background(), task.ID)

	require.NoError(t, err, "a failed launch ends the task in failed, it is not returned")
	assert.Empty(t, fx.repo.pinned)
	assert.Nil(t, fx.repo.tasks[task.ID].Agent.Runtime)
}
