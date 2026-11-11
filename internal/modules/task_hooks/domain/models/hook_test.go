package models_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
)

func scriptParams() models.HookParams {
	return models.HookParams{
		ID: "h1", Name: "Lint", Kind: models.KindScript, Timeout: time.Minute,
		Script: models.ScriptSpec{Command: " make ", Args: []string{"test", " "}},
	}
}

func agentParams() models.HookParams {
	return models.HookParams{
		ID: "h2", Name: "Review", Kind: models.KindAgent, Timeout: 5 * time.Minute,
		Agent: models.AgentSpec{Prompt: " review the diff ", Runtime: " claude ", Model: " m ", Thinking: " high ", Profile: " p "},
	}
}

func TestNewHookScriptTrimsAndKeepsItsSpec(t *testing.T) {
	t.Parallel()
	hook, err := models.NewHook(models.HookParams{
		ID: "h1", Name: "  Lint  ", Kind: models.KindScript, Timeout: time.Minute,
		Script: models.ScriptSpec{
			Command: " make ", Args: []string{"test", " ", ""}, Env: []models.EnvVar{{Name: " CI ", Value: "1"}},
			UseShell: true, WorkingDir: " sub/dir ",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "h1", hook.ID())
	assert.Equal(t, "Lint", hook.Name())
	assert.Equal(t, models.KindScript, hook.Kind())
	assert.Equal(t, time.Minute, hook.Timeout())
	assert.Equal(t, models.ScriptSpec{
		Command: "make", Args: []string{"test"}, Env: []models.EnvVar{{Name: "CI", Value: "1"}},
		UseShell: true, WorkingDir: "sub/dir",
	}, hook.Script())
	assert.Equal(t, models.AgentSpec{}, hook.Agent(), "a script hook keeps no agent spec")
}

func TestNewHookAgentTrimsAndKeepsOnlyItsSpec(t *testing.T) {
	t.Parallel()
	hook, err := models.NewHook(agentParams())

	require.NoError(t, err)
	assert.Equal(t, models.KindAgent, hook.Kind())
	assert.Equal(t, models.AgentSpec{Prompt: "review the diff", Runtime: "claude", Model: "m", Thinking: "high", Profile: "p"},
		hook.Agent())
	assert.Empty(t, hook.Script().Command, "an agent hook keeps no script spec")
}

func TestNewHookTimeoutBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{name: "one second", timeout: time.Second},
		{name: "exactly the maximum", timeout: models.MaxTimeout},
		{name: "zero", timeout: 0, wantErr: true},
		{name: "negative", timeout: -time.Second, wantErr: true},
		{name: "above the maximum", timeout: models.MaxTimeout + time.Second, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := scriptParams()
			p.Timeout = tc.timeout

			hook, err := models.NewHook(p)

			if tc.wantErr {
				assert.True(t, errors.Is(err, hookerrors.ErrInvalidHook), "got %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.timeout, hook.Timeout())
		})
	}
}

func TestNewHookInvalid(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		base   func() models.HookParams
		mutate func(*models.HookParams)
	}{
		"blank id":            {scriptParams, func(p *models.HookParams) { p.ID = " " }},
		"blank name":          {scriptParams, func(p *models.HookParams) { p.Name = "  " }},
		"unknown kind":        {scriptParams, func(p *models.HookParams) { p.Kind = "other" }},
		"empty kind":          {scriptParams, func(p *models.HookParams) { p.Kind = "" }},
		"blank command":       {scriptParams, func(p *models.HookParams) { p.Script.Command = " " }},
		"nul in command":      {scriptParams, func(p *models.HookParams) { p.Script.Command = "ma\x00ke" }},
		"nul in working dir":  {scriptParams, func(p *models.HookParams) { p.Script.WorkingDir = "a\x00b" }},
		"env name with equal": {scriptParams, func(p *models.HookParams) { p.Script.Env = []models.EnvVar{{Name: "A=B"}} }},
		"env name with space": {scriptParams, func(p *models.HookParams) { p.Script.Env = []models.EnvVar{{Name: "A B"}} }},
		"blank env name":      {scriptParams, func(p *models.HookParams) { p.Script.Env = []models.EnvVar{{Name: " "}} }},
		"blank prompt":        {agentParams, func(p *models.HookParams) { p.Agent.Prompt = " \t " }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := tc.base()
			tc.mutate(&p)

			_, err := models.NewHook(p)

			assert.True(t, errors.Is(err, hookerrors.ErrInvalidHook), "got %v", err)
		})
	}
}

func TestNewHookRejectsTheWrongSpecOfItsKind(t *testing.T) {
	t.Parallel()
	// A script without a command is invalid even when an agent prompt is present, and the other way around.
	p := agentParams()
	p.Kind = models.KindScript
	p.Script.Command = ""

	_, err := models.NewHook(p)

	assert.True(t, errors.Is(err, hookerrors.ErrInvalidHook), "got %v", err)
}

func TestHookParamsRoundTrip(t *testing.T) {
	t.Parallel()
	for name, params := range map[string]models.HookParams{"script": scriptParams(), "agent": agentParams()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hook, err := models.NewHook(params)
			require.NoError(t, err)

			again, err := models.NewHook(hook.Params())

			require.NoError(t, err)
			assert.Equal(t, hook, again)
		})
	}
}

func TestHookScriptCopyDoesNotAliasTheHook(t *testing.T) {
	t.Parallel()
	hook, err := models.NewHook(scriptParams())
	require.NoError(t, err)

	spec := hook.Script()
	spec.Args[0] = "changed"

	assert.Equal(t, "test", hook.Script().Args[0])
}
