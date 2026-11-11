package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

const (
	testVerdictInstruction = "`VERDICT: OK`"
	testMissingVerdict     = "[no VERDICT line in the agent answer]"
)

func agentHook(t *testing.T, _ models.Phase, prompt string, timeout time.Duration) models.Hook {
	t.Helper()
	hook, err := models.NewHook(models.HookParams{
		ID: "h1", Name: "agent", Kind: models.KindAgent, Timeout: timeout,
		Agent: models.AgentSpec{Prompt: prompt},
	})
	require.NoError(t, err)
	return hook
}

func agentRun(phase models.Phase) models.RunContext {
	return models.RunContext{
		TaskID: "t1", TaskTitle: "Add login", Branch: "feat/login", WorktreePath: "/wt", Phase: phase,
		Event: models.EventTaskStart,
	}
}

func TestAgentHookRunnerVerdicts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		phase      models.Phase
		session    fakeAgentSession
		timeout    time.Duration
		wantStatus models.RunStatus
		wantOutput string
	}{
		{
			name: "OK verdict on the last line passes a pre hook", phase: models.PhasePre,
			session:    fakeAgentSession{result: outbound.AgentSessionResult{Text: "checked\nVERDICT: OK"}},
			wantStatus: models.RunOK, wantOutput: "checked\nVERDICT: OK",
		},
		{
			name: "FAIL verdict with a reason fails a pre hook", phase: models.PhasePre,
			session:    fakeAgentSession{result: outbound.AgentSessionResult{Text: "tests are red\nVERDICT: FAIL tests are red"}},
			wantStatus: models.RunFailed, wantOutput: "tests are red\nVERDICT: FAIL tests are red",
		},
		{
			name: "the last verdict line wins", phase: models.PhasePre,
			session:    fakeAgentSession{result: outbound.AgentSessionResult{Text: "VERDICT: FAIL\nfixed\nVERDICT: OK"}},
			wantStatus: models.RunOK, wantOutput: "VERDICT: FAIL\nfixed\nVERDICT: OK",
		},
		{
			name: "markdown emphasis around the verdict is ignored", phase: models.PhasePre,
			session:    fakeAgentSession{result: outbound.AgentSessionResult{Text: "**VERDICT: OK**"}},
			wantStatus: models.RunOK, wantOutput: "**VERDICT: OK**",
		},
		{
			name: "an unknown verdict word counts as missing and fails a pre hook", phase: models.PhasePre,
			session:    fakeAgentSession{result: outbound.AgentSessionResult{Text: "VERDICT: MAYBE"}},
			wantStatus: models.RunFailed, wantOutput: "VERDICT: MAYBE\n" + testMissingVerdict,
		},
		{
			name: "a missing verdict fails a pre hook", phase: models.PhasePre,
			session:    fakeAgentSession{result: outbound.AgentSessionResult{Text: "all good"}},
			wantStatus: models.RunFailed, wantOutput: "all good\n" + testMissingVerdict,
		},
		{
			name: "a missing verdict passes a post hook", phase: models.PhasePost,
			session:    fakeAgentSession{result: outbound.AgentSessionResult{Text: "all good"}},
			wantStatus: models.RunOK, wantOutput: "all good",
		},
		{
			name: "an empty answer passes a post hook", phase: models.PhasePost,
			session:    fakeAgentSession{},
			wantStatus: models.RunOK, wantOutput: "",
		},
		{
			name: "a FAIL verdict fails a post hook", phase: models.PhasePost,
			session:    fakeAgentSession{result: outbound.AgentSessionResult{Text: "VERDICT: FAIL lint"}},
			wantStatus: models.RunFailed, wantOutput: "VERDICT: FAIL lint",
		},
		{
			name: "a failed session fails the hook with its reason", phase: models.PhasePost,
			session: fakeAgentSession{result: outbound.AgentSessionResult{
				Text: "partial", Failed: true, FailureReason: "agent error",
			}},
			wantStatus: models.RunFailed, wantOutput: "partial\n[agent session failed: agent error]",
		},
		{
			name: "a session that outlives the hook timeout times out", phase: models.PhasePre,
			session: fakeAgentSession{wait: true}, timeout: 20 * time.Millisecond,
			wantStatus: models.RunTimedOut, wantOutput: "the agent did not finish within 20ms",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			timeout := tc.timeout
			if timeout == 0 {
				timeout = time.Minute
			}
			session := tc.session
			runner := application.NewAgentHookRunner(&session)

			outcome, err := runner.Run(context.Background(), agentHook(t, tc.phase, "check it", timeout), agentRun(tc.phase))

			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, outcome.Status)
			assert.Equal(t, -1, outcome.ExitCode)
			assert.Equal(t, tc.wantOutput, outcome.Output)
		})
	}
}

func TestAgentHookRunnerExpandsPromptAndPassesOverrides(t *testing.T) {
	t.Parallel()
	session := &fakeAgentSession{result: outbound.AgentSessionResult{Text: "VERDICT: OK"}}
	hook, err := models.NewHook(models.HookParams{
		ID: "h1", Name: "agent", Kind: models.KindAgent, Timeout: time.Minute,
		Agent: models.AgentSpec{Prompt: "Review {{.Branch}} for {{.TaskTitle}}", Runtime: "claude", Model: "m", Thinking: "high", Profile: "p"},
	})
	require.NoError(t, err)

	outcome, err := application.NewAgentHookRunner(session).Run(context.Background(), hook, agentRun(models.PhasePre))

	require.NoError(t, err)
	assert.Equal(t, models.RunOK, outcome.Status)
	require.Len(t, session.requests, 1)
	request := session.requests[0]
	assert.Equal(t, "/wt", request.Dir)
	assert.True(t, strings.HasPrefix(request.Prompt, "Review feat/login for Add login\n\n"), request.Prompt)
	assert.Contains(t, request.Prompt, testVerdictInstruction)
	assert.Equal(t, "claude", request.Runtime)
	assert.Equal(t, "m", request.Model)
	assert.Equal(t, "high", request.Thinking)
	assert.Equal(t, "p", request.Profile)
}

func TestAgentHookRunnerRejectsBadTemplateWithoutStartingSession(t *testing.T) {
	t.Parallel()
	session := &fakeAgentSession{}
	hook := agentHook(t, models.PhasePre, "check {{.Unknown}}", time.Minute)

	_, err := application.NewAgentHookRunner(session).Run(context.Background(), hook, agentRun(models.PhasePre))

	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
	assert.Empty(t, session.requests)
}

func TestAgentHookRunnerReportsSessionThatCouldNotStart(t *testing.T) {
	t.Parallel()
	session := &fakeAgentSession{err: errors.New("no runtime")}

	_, err := application.NewAgentHookRunner(session).Run(context.Background(), agentHook(t, models.PhasePre, "check", time.Minute), agentRun(models.PhasePre))

	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
}

func TestAgentHookRunnerBoundsTheAnswerButKeepsTheVerdict(t *testing.T) {
	t.Parallel()
	const limit = 1 << 20
	text := strings.Repeat("é", limit) + "\nVERDICT: OK"
	session := &fakeAgentSession{result: outbound.AgentSessionResult{Text: text}}

	outcome, err := application.NewAgentHookRunner(session).Run(context.Background(), agentHook(t, models.PhasePre, "check", time.Minute), agentRun(models.PhasePre))

	require.NoError(t, err)
	assert.Equal(t, models.RunOK, outcome.Status)
	assert.Contains(t, outcome.Output, "[output truncated:")
	assert.LessOrEqual(t, len(outcome.Output), limit+100)
	assert.True(t, strings.HasPrefix(outcome.Output, "é"))
}
