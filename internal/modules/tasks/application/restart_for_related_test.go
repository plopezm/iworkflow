package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
)

// relatedSection is the section about one project a notice carries after the intro.
const relatedSection = "## Related projects\n- web: /repos/web"

// flushRecorder stands in for the event queue of the restart tests and notes each flush in the call log.
type flushRecorder struct {
	runtime *resumeRuntime
}

// Flush notes the flush in the call log.
func (f flushRecorder) Flush() {
	f.runtime.note("flush")
}

// restarter builds the restarter over the fixture; the test picks the sink, the flusher and the guard.
func (f *resumeFixture) restarter(sink inbound.AgentEventSink, flusher application.EventFlusher, guard *application.RestartGuard) *application.RelatedProjectsRestarter {
	related := application.NewPrepareRelatedLaunch(application.NewListTaskRelatedProjects(f.related, relatedWorkspaces()), f.probe)
	return application.NewRelatedProjectsRestarter(f.lifecycle, f.worktrees, f.runtime, f.settings, sink, related, flusher, guard)
}

// promptSent returns the message of the only command the agent received, which must be a prompt.
func promptSent(t *testing.T, sent []sentCommand) string {
	t.Helper()
	require.Len(t, sent, 1)
	prompt, ok := sent[0].command.(models.PromptCommand)
	require.True(t, ok, "an idle agent gets the notice as a prompt")
	return prompt.Message
}

func TestRelatedRestartRestartsAnIdleAgentWithResumeAndThenTellsIt(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.related.stored[task.ID.String()] = []string{"ws-api"}
	restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())
	defer restarter.Close()

	outcome, err := restarter.Request(context.Background(), task.ID, relatedSection)

	require.NoError(t, err)
	assert.Equal(t, application.RestartOutcome{Restarted: true}, outcome)
	assert.Equal(t, []string{"stop", "flush", "start", "send"}, fx.runtime.calls, "the agent stops, its exit is processed, it resumes, and only then gets the notice")
	require.Len(t, fx.runtime.started, 1)
	assert.True(t, fx.runtime.started[0].resume, "the agent resumes its session")
	assert.Equal(t, []string{"/repos/api"}, fx.runtime.started[0].options.AddDirs, "the resumed agent may read the folder that was added")
	message := promptSent(t, fx.runtime.sent)
	assert.True(t, strings.HasPrefix(message, "The user has just related more projects"), message)
	assert.True(t, strings.HasSuffix(message, relatedSection), message)
	assert.Equal(t, models.StatusIdle, fx.repo.tasks[task.ID].Status)
}

func TestRelatedRestartWaitsForARunningAgentAndRestartsItOnceWhenIdle(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.related.stored[task.ID.String()] = []string{"ws-api"}
	restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())

	outcome, err := restarter.Request(context.Background(), task.ID, relatedSection)

	require.NoError(t, err)
	assert.Equal(t, application.RestartOutcome{Pending: true}, outcome)
	assert.Empty(t, fx.runtime.calls, "a running agent is not touched")

	// The agent settles: the event use case records the idle status and then calls the hook.
	require.NoError(t, fx.lifecycle.Transition(context.Background(), task.ID, models.StatusIdle, nil))
	restarter.TaskIdle(task.ID)
	restarter.Close()

	assert.Equal(t, []string{"stop", "flush", "start", "send"}, fx.runtime.calls, "the agent is restarted exactly once")
	assert.True(t, strings.HasSuffix(promptSent(t, fx.runtime.sent), relatedSection))
}

func TestRelatedRestartIdleHookDoesNothingWithoutANotice(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())

	restarter.TaskIdle(task.ID)
	restarter.Close()

	assert.Empty(t, fx.runtime.calls)
}

func TestRelatedRestartMergesTheNoticesOfABusyAgentIntoOneRestart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())
	first := "## Related projects\n- api: /repos/api"
	second := "## Related projects\n- web: /repos/web"

	for _, section := range []string{first, second} {
		outcome, err := restarter.Request(context.Background(), task.ID, section)
		require.NoError(t, err)
		assert.Equal(t, application.RestartOutcome{Pending: true}, outcome)
	}
	require.NoError(t, fx.lifecycle.Transition(context.Background(), task.ID, models.StatusIdle, nil))
	restarter.TaskIdle(task.ID)
	restarter.Close()

	assert.Equal(t, []string{"stop", "flush", "start", "send"}, fx.runtime.calls, "two notices, one restart")
	message := promptSent(t, fx.runtime.sent)
	assert.Equal(t, 1, strings.Count(message, "The user has just related"), "the intro is sent once")
	assert.True(t, strings.HasSuffix(message, first+"\n\n"+second), message)
}

func TestRelatedRestartLeavesTheAgentAloneWhenItCannotResume(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(fx *resumeFixture)
	}{
		{name: "the runtime cannot resume a session", setup: func(fx *resumeFixture) { fx.runtime.capabilities = models.AgentCapabilities{} }},
		{name: "the worktree is gone", setup: func(fx *resumeFixture) { fx.worktrees.exists = false }},
		{name: "the launch settings are unavailable", setup: func(fx *resumeFixture) { fx.settings.err = werrors.Runtime("ajustes no disponibles") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusIdle)
			fx := newResumeFixture(task)
			fx.runtime.running[task.ID] = true
			fx.related.stored[task.ID.String()] = []string{"ws-api"}
			tt.setup(fx)
			restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())

			outcome, err := restarter.Request(context.Background(), task.ID, relatedSection)

			require.NoError(t, err)
			assert.Equal(t, application.RestartOutcome{}, outcome)
			assert.Empty(t, fx.runtime.calls, "nothing is stopped, started or sent")
			assert.Empty(t, fx.repo.statuses, "the task keeps its status")
			assert.Equal(t, []string{"ws-api"}, fx.related.stored[task.ID.String()], "the save is untouched")
		})
	}
}

func TestRelatedRestartMarksTheTaskFailedWhenTheAgentCannotBeRestarted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setup     func(fx *resumeFixture, id models.TaskID)
		wantCalls []string
		wantError string
	}{
		{
			name:      "the agent does not stop",
			setup:     func(fx *resumeFixture, _ models.TaskID) { fx.runtime.stopErr = errors.New("the agent did not stop") },
			wantCalls: []string{"stop", "flush"},
			wantError: "the agent did not stop",
		},
		{
			name: "the agent does not start again",
			setup: func(fx *resumeFixture, id models.TaskID) {
				fx.runtime.startErrs[id] = errors.New("the agent did not start")
			},
			wantCalls: []string{"stop", "flush", "start"},
			wantError: "the agent did not start",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusIdle)
			fx := newResumeFixture(task)
			fx.runtime.running[task.ID] = true
			fx.related.stored[task.ID.String()] = []string{"ws-api"}
			tt.setup(fx, task.ID)
			restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())

			outcome, err := restarter.Request(context.Background(), task.ID, relatedSection)

			require.NoError(t, err, "the failure is recorded in the task, not returned")
			assert.Equal(t, application.RestartOutcome{}, outcome)
			assert.Equal(t, tt.wantCalls, fx.runtime.calls)
			stored := fx.repo.tasks[task.ID]
			assert.Equal(t, models.StatusFailed, stored.Status)
			require.NotNil(t, stored.Error)
			assert.Equal(t, tt.wantError, *stored.Error)
		})
	}
}

func TestRelatedRestartKeepsTheTaskIdleWhenTheStoppedAgentExitsThroughTheQueue(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.related.stored[task.ID.String()] = []string{"ws-api"}
	guard := application.NewRestartGuard()
	agentEvents := application.NewHandleAgentEvent(fx.lifecycle, clock)
	agentEvents.SetRestartGuard(guard)
	queue := application.NewQueuedAgentEventSink(agentEvents, 16)
	defer queue.Shutdown()
	fx.runtime.exitTo = queue
	restarter := fx.restarter(queue, queue, guard)
	defer restarter.Close()

	outcome, err := restarter.Request(context.Background(), task.ID, relatedSection)

	require.NoError(t, err)
	assert.True(t, outcome.Restarted)
	assert.NotContains(t, fx.repo.statuses, models.StatusInterrupted, "the exit of the restarted agent is not a crash")
	assert.Equal(t, models.StatusIdle, fx.repo.tasks[task.ID].Status)
	assert.Equal(t, []string{"stop", "start", "send"}, fx.runtime.calls, "the queue processes the exit before the restart goes on")
}

func TestRelatedRestartFlushesTheEventsBeforeTheAgentStarts(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.related.stored[task.ID.String()] = []string{"ws-api"}
	restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())
	defer restarter.Close()

	_, err := restarter.Request(context.Background(), task.ID, relatedSection)

	require.NoError(t, err)
	assert.Equal(t, []string{"stop", "flush", "start", "send"}, fx.runtime.calls)
}

func TestRelatedRestartCloseWaitsForTheRestartItStarted(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.related.stored[task.ID.String()] = []string{"ws-api"}
	fx.runtime.startEntered = make(chan struct{})
	fx.runtime.startGate = make(chan struct{})
	restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())
	_, err := restarter.Request(context.Background(), task.ID, relatedSection)
	require.NoError(t, err)
	require.NoError(t, fx.lifecycle.Transition(context.Background(), task.ID, models.StatusIdle, nil))
	restarter.TaskIdle(task.ID)
	waitForSignal(t, fx.runtime.startEntered)

	closed := make(chan struct{})
	go func() {
		restarter.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned while the restart was still starting the agent")
	case <-time.After(50 * time.Millisecond):
	}
	close(fx.runtime.startGate)
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return after the restart finished")
	}

	assert.Equal(t, []string{"stop", "flush", "start", "send"}, fx.runtime.calls, "the restart finished before Close returned")
}

func TestRelatedRestartRefusesNoticesAfterClose(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())
	restarter.Close()

	_, err := restarter.Request(context.Background(), task.ID, relatedSection)

	require.Error(t, err)
	restarter.TaskIdle(task.ID)
	assert.Empty(t, fx.runtime.calls, "a closed restarter starts nothing")
}

func TestSetTaskRelatedProjectsRemovalOnlySavesWithARealRestarter(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.related.stored[task.ID.String()] = []string{relatedAPI, relatedWeb}
	known := uuidWorkspaces()
	lifecycle := application.NewTaskLifecycle(fx.repo, &fakePublisher{}, known, clock)
	restarter := fx.restarter(fx.sink, flushRecorder{runtime: fx.runtime}, application.NewRestartGuard())
	defer restarter.Close()
	useCase := application.NewSetTaskRelatedProjects(lifecycle, fx.related, application.NewListTaskRelatedProjects(fx.related, known), fx.probe, restarter)

	result, err := useCase.Execute(context.Background(), task.ID, []string{relatedWeb})

	require.NoError(t, err)
	assert.Equal(t, application.SetRelatedProjectsResult{}, result)
	assert.Equal(t, []string{relatedWeb}, fx.related.stored[task.ID.String()])
	assert.Empty(t, fx.runtime.calls, "removing a project never stops or starts the agent")
}

func TestHandleAgentEventCallsTheIdleHookOnlyWhenATaskBecomesIdle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    models.TaskStatus
		event     models.AgentEvent
		wantCalls int
	}{
		{name: "a settled run makes a running task idle", status: models.StatusRunning, event: models.RunSettled{}, wantCalls: 1},
		{name: "a settled run makes a starting task idle", status: models.StatusStarting, event: models.RunSettled{}, wantCalls: 1},
		{name: "a task already idle does not change", status: models.StatusIdle, event: models.RunSettled{}, wantCalls: 0},
		{name: "a backlog task does not change", status: models.StatusBacklog, event: models.RunSettled{}, wantCalls: 0},
		{name: "a run that starts is not idle", status: models.StatusStarting, event: models.RunStarted{}, wantCalls: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(tt.status)
			lifecycle, _ := newLifecycle(newFakeRepository(task))
			calls := 0
			events := application.NewHandleAgentEvent(lifecycle, clock)
			events.SetIdleHook(func(models.TaskID) { calls++ })

			require.NoError(t, events.Execute(context.Background(), task.ID, tt.event))

			assert.Equal(t, tt.wantCalls, calls)
		})
	}
}

func TestHandleAgentEventIgnoresTheExitOfAnAgentBeingRestarted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		guarded    bool
		event      models.AgentEvent
		wantStatus models.TaskStatus
		dropped    bool
	}{
		{name: "an exit with no restart under way interrupts the running task", event: models.ProcessExited{}, wantStatus: models.StatusInterrupted},
		{name: "an exit of an agent being restarted is dropped", guarded: true, event: models.ProcessExited{}, wantStatus: models.StatusRunning, dropped: true},
		{name: "other events keep their effect during a restart", guarded: true, event: models.RunSettled{}, wantStatus: models.StatusIdle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusRunning)
			repo := newFakeRepository(task)
			lifecycle, publisher := newLifecycle(repo)
			guard := application.NewRestartGuard()
			if tt.guarded {
				guard.Mark(task.ID)
			}
			events := application.NewHandleAgentEvent(lifecycle, clock)
			events.SetRestartGuard(guard)

			require.NoError(t, events.Execute(context.Background(), task.ID, tt.event))

			assert.Equal(t, tt.wantStatus, repo.tasks[task.ID].Status)
			if tt.dropped {
				assert.Empty(t, repo.statuses)
				assert.Empty(t, publisher.published, "a dropped event is neither recorded nor published")
			}
		})
	}
}
