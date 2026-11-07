package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	hooksoutbound "iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	tasksinbound "iworkflow/internal/modules/tasks/domain/ports/inbound"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// fakeHookRuntime is an agent runtime that delivers scripted events when the prompt arrives. Nothing real is
// launched: the session is only observed through the calls it receives.
type fakeHookRuntime struct {
	events     []tasksmodels.AgentEvent
	startErr   error
	sendErr    error
	started    int
	cwd        string
	options    tasksmodels.LaunchOptions
	prompt     string
	taskID     tasksmodels.TaskID
	stops      int
	stopCtxErr error
}

var _ tasksoutbound.AgentRuntime = (*fakeHookRuntime)(nil)

func (f *fakeHookRuntime) Start(_ context.Context, taskID tasksmodels.TaskID, cwd string, _ bool, options tasksmodels.LaunchOptions, _ tasksinbound.AgentEventSink) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started++
	f.taskID, f.cwd, f.options = taskID, cwd, options
	return nil
}

func (f *fakeHookRuntime) Send(_ context.Context, _ tasksmodels.TaskID, command tasksmodels.AgentCommand) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	prompt, isPrompt := command.(tasksmodels.PromptCommand)
	if !isPrompt {
		return errors.New("unexpected command")
	}
	f.prompt = prompt.Message
	return nil
}

func (f *fakeHookRuntime) Stop(ctx context.Context, _ tasksmodels.TaskID) error {
	f.stops++
	f.stopCtxErr = ctx.Err()
	return nil
}

func (f *fakeHookRuntime) StopAll(context.Context) {}

func (f *fakeHookRuntime) IsRunning(context.Context, tasksmodels.TaskID) bool { return false }

func (f *fakeHookRuntime) Subagents(context.Context, tasksmodels.TaskID) []tasksmodels.SubagentTask {
	return nil
}

func (f *fakeHookRuntime) Capabilities(tasksmodels.TaskID) tasksmodels.AgentCapabilities {
	return tasksmodels.FullAgentCapabilities()
}

func (f *fakeHookRuntime) RuntimeCapabilities(*tasksmodels.RuntimeID) tasksmodels.AgentCapabilities {
	return tasksmodels.FullAgentCapabilities()
}

// fakeLaunchSettings records the overrides the session asks for and returns them as the launch options.
type fakeLaunchSettings struct {
	got tasksmodels.TaskAgentOptions
}

var _ tasksoutbound.LaunchSettings = (*fakeLaunchSettings)(nil)

func (f *fakeLaunchSettings) Prepare(_ context.Context, _ string, agent tasksmodels.TaskAgentOptions) (tasksmodels.LaunchOptions, error) {
	f.got = agent
	options := tasksmodels.LaunchOptions{}
	if agent.Runtime != nil {
		runtime := *agent.Runtime
		options.Runtime = &runtime
	}
	return options, nil
}

// sinkRuntime delivers the scripted events through the sink of the session when the prompt is sent, so they arrive
// after Start, as they do for a real agent.
type sinkRuntime struct {
	*fakeHookRuntime
	sink tasksinbound.AgentEventSink
}

func (r *sinkRuntime) Start(ctx context.Context, taskID tasksmodels.TaskID, cwd string, resume bool, options tasksmodels.LaunchOptions, sink tasksinbound.AgentEventSink) error {
	r.sink = sink
	return r.fakeHookRuntime.Start(ctx, taskID, cwd, resume, options, sink)
}

func (r *sinkRuntime) Send(ctx context.Context, taskID tasksmodels.TaskID, command tasksmodels.AgentCommand) error {
	if err := r.fakeHookRuntime.Send(ctx, taskID, command); err != nil {
		return err
	}
	for _, event := range r.events {
		r.sink.Handle(ctx, taskID, event)
	}
	return nil
}

func hookRequest(t *testing.T) hooksoutbound.AgentSessionRequest {
	t.Helper()
	return hooksoutbound.AgentSessionRequest{Dir: t.TempDir(), Prompt: "check the branch"}
}

func newHookSession(runtime *sinkRuntime, settings *fakeLaunchSettings) hooksoutbound.AgentSession {
	return NewTaskHooksAgentSession(settings, runtime)
}

func TestTaskHooksAgentSessionReturnsTheLastAnswerWhenTheTurnSettles(t *testing.T) {
	t.Parallel()
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{events: []tasksmodels.AgentEvent{
		tasksmodels.RunStarted{},
		tasksmodels.AssistantMessage{Text: "first pass"},
		tasksmodels.AssistantMessage{Text: "VERDICT: OK"},
		tasksmodels.RunSettled{},
	}}}
	settings := &fakeLaunchSettings{}
	request := hookRequest(t)

	result, err := newHookSession(runtime, settings).Run(context.Background(), request)

	require.NoError(t, err)
	assert.Equal(t, hooksoutbound.AgentSessionResult{Text: "VERDICT: OK"}, result)
	assert.Equal(t, request.Dir, runtime.cwd, "the session runs in the directory of the task")
	assert.Equal(t, request.Prompt, runtime.prompt)
	assert.NotEqual(t, tasksmodels.TaskID{}, runtime.taskID, "the session has its own id")
	assert.Equal(t, 1, runtime.stops)
	assert.NoError(t, runtime.stopCtxErr, "the stop must not be cut short by the session context")
}

func TestTaskHooksAgentSessionFailsWhenTheProcessExitsBeforeSettling(t *testing.T) {
	t.Parallel()
	code := int32(1)
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{events: []tasksmodels.AgentEvent{
		tasksmodels.AssistantMessage{Text: "partial"},
		tasksmodels.ProcessExited{Code: &code},
	}}}

	result, err := newHookSession(runtime, &fakeLaunchSettings{}).Run(context.Background(), hookRequest(t))

	require.NoError(t, err)
	assert.True(t, result.Failed)
	assert.Equal(t, "partial", result.Text)
	assert.Contains(t, result.FailureReason, "exited before it finished")
	assert.Equal(t, 1, runtime.stops)
}

func TestTaskHooksAgentSessionFailsAtOnceOnADialogNobodyCanAnswer(t *testing.T) {
	t.Parallel()
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{events: []tasksmodels.AgentEvent{
		tasksmodels.DialogRequested{Request: tasksmodels.DialogRequest{ID: "d1", Title: "Overwrite the file?"}},
		tasksmodels.RunSettled{},
	}}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := newHookSession(runtime, &fakeLaunchSettings{}).Run(ctx, hookRequest(t))

	require.NoError(t, err, "a dialog must end the session, not hang it until the deadline")
	assert.True(t, result.Failed)
	assert.Contains(t, result.FailureReason, "Overwrite the file?")
	assert.Equal(t, 1, runtime.stops)
}

func TestTaskHooksAgentSessionFailsOnAnErrorNoticeEvenWhenItSettles(t *testing.T) {
	t.Parallel()
	failure := "provider rejected the request"
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{events: []tasksmodels.AgentEvent{
		tasksmodels.Notice{Level: tasksmodels.NoticeError, Message: failure},
		tasksmodels.AssistantMessage{Error: &failure},
		tasksmodels.RunSettled{},
	}}}

	result, err := newHookSession(runtime, &fakeLaunchSettings{}).Run(context.Background(), hookRequest(t))

	require.NoError(t, err)
	assert.True(t, result.Failed)
	assert.Equal(t, failure, result.FailureReason)
}

func TestTaskHooksAgentSessionTimesOutAndStopsTheAgent(t *testing.T) {
	t.Parallel()
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := newHookSession(runtime, &fakeLaunchSettings{}).Run(ctx, hookRequest(t))

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, runtime.stops, "a session that outlives its deadline is still stopped")
	assert.NoError(t, runtime.stopCtxErr, "the stop must not be cut short by the expired context")
}

func TestTaskHooksAgentSessionReportsAFailedPromptAsAFailedSession(t *testing.T) {
	t.Parallel()
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{sendErr: errors.New("agent not running")}}

	result, err := newHookSession(runtime, &fakeLaunchSettings{}).Run(context.Background(), hookRequest(t))

	require.NoError(t, err)
	assert.True(t, result.Failed)
	assert.Contains(t, result.FailureReason, "agent not running")
	assert.Equal(t, 1, runtime.stops)
}

func TestTaskHooksAgentSessionRefusesToStartWithoutAttempting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		request func(t *testing.T) hooksoutbound.AgentSessionRequest
	}{
		{
			name: "missing directory",
			request: func(t *testing.T) hooksoutbound.AgentSessionRequest {
				t.Helper()
				return hooksoutbound.AgentSessionRequest{Dir: t.TempDir() + "/missing", Prompt: "p"}
			},
		},
		{
			name: "unknown runtime",
			request: func(t *testing.T) hooksoutbound.AgentSessionRequest {
				t.Helper()
				request := hookRequest(t)
				request.Runtime = "not-a-runtime"
				return request
			},
		},
		{
			name: "runtime without a checked text mapping",
			request: func(t *testing.T) hooksoutbound.AgentSessionRequest {
				t.Helper()
				request := hookRequest(t)
				request.Runtime = "agy"
				return request
			},
		},
		{
			name: "invalid thinking level",
			request: func(t *testing.T) hooksoutbound.AgentSessionRequest {
				t.Helper()
				request := hookRequest(t)
				request.Thinking = "extreme-plus"
				return request
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{}}

			_, err := newHookSession(runtime, &fakeLaunchSettings{}).Run(context.Background(), tc.request(t))

			require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
			assert.Equal(t, 0, runtime.started, "the agent must not start")
			assert.Equal(t, 0, runtime.stops)
		})
	}
}

func TestTaskHooksAgentSessionReportsAFailedStartAsNotStarted(t *testing.T) {
	t.Parallel()
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{startErr: errors.New("no such binary")}}

	_, err := newHookSession(runtime, &fakeLaunchSettings{}).Run(context.Background(), hookRequest(t))

	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
	assert.Equal(t, 0, runtime.stops, "nothing was started, so nothing is stopped")
}

func TestTaskHooksAgentSessionPassesTheOverridesToTheLaunch(t *testing.T) {
	t.Parallel()
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{events: []tasksmodels.AgentEvent{
		tasksmodels.AssistantMessage{Text: "VERDICT: OK"},
		tasksmodels.RunSettled{},
	}}}
	settings := &fakeLaunchSettings{}
	request := hookRequest(t)
	request.Runtime = "claude"
	request.Model = "anthropic/claude-x"
	request.Thinking = "high"
	request.Profile = "work"

	_, err := newHookSession(runtime, settings).Run(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, settings.got.Runtime)
	assert.Equal(t, "claude", settings.got.Runtime.String())
	require.NotNil(t, settings.got.ModelProvider)
	assert.Equal(t, "anthropic", *settings.got.ModelProvider)
	require.NotNil(t, settings.got.ModelID)
	assert.Equal(t, "claude-x", *settings.got.ModelID)
	require.NotNil(t, settings.got.Thinking)
	assert.Equal(t, "high", *settings.got.Thinking)
	require.NotNil(t, settings.got.Profile)
	assert.Equal(t, "work", *settings.got.Profile)
	require.NotNil(t, runtime.options.Runtime)
	assert.Equal(t, "claude", runtime.options.Runtime.String())
}

func TestTaskHooksAgentSessionUsesTheWorkspaceDefaultWithoutOverrides(t *testing.T) {
	t.Parallel()
	runtime := &sinkRuntime{fakeHookRuntime: &fakeHookRuntime{events: []tasksmodels.AgentEvent{
		tasksmodels.AssistantMessage{Text: "VERDICT: OK"},
		tasksmodels.RunSettled{},
	}}}
	settings := &fakeLaunchSettings{}

	_, err := newHookSession(runtime, settings).Run(context.Background(), hookRequest(t))

	require.NoError(t, err)
	assert.Nil(t, settings.got.Runtime, "no override means the default runtime of the launch settings")
	assert.Nil(t, settings.got.ModelID)
	assert.Nil(t, settings.got.ModelProvider)
	assert.Nil(t, settings.got.Thinking)
	assert.Nil(t, settings.got.Profile)
	assert.Nil(t, runtime.options.Runtime)
}
