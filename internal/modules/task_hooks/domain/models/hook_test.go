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

func script() models.HookParams {
	return models.HookParams{
		ID: "h1", WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureBlock, Timeout: time.Minute,
		Script: models.ScriptSpec{Command: " make ", Args: []string{"test", " "}},
	}
}

func TestNewHookValid(t *testing.T) {
	hook, err := models.NewHook(script())
	require.NoError(t, err)
	assert.Equal(t, "make", hook.Script().Command)
	assert.Equal(t, []string{"test"}, hook.Script().Args)
	assert.Equal(t, models.AgentSpec{}, hook.Agent())
}

func TestNewHookAgentKeepsOnlyAgentSpec(t *testing.T) {
	p := script()
	p.Kind = models.KindAgent
	p.Agent = models.AgentSpec{Prompt: " run the linter ", Model: " m "}
	hook, err := models.NewHook(p)
	require.NoError(t, err)
	assert.Equal(t, "run the linter", hook.Agent().Prompt)
	assert.Equal(t, "m", hook.Agent().Model)
	assert.Empty(t, hook.Script().Command)
}

func TestNewHookInvalid(t *testing.T) {
	cases := map[string]func(*models.HookParams){
		"id":          func(p *models.HookParams) { p.ID = " " },
		"workspace":   func(p *models.HookParams) { p.WorkspaceID = "" },
		"phase":       func(p *models.HookParams) { p.Phase = "during" },
		"event":       func(p *models.HookParams) { p.Event = "task_discard" },
		"kind":        func(p *models.HookParams) { p.Kind = "other" },
		"failure":     func(p *models.HookParams) { p.OnFailure = "" },
		"no timeout":  func(p *models.HookParams) { p.Timeout = 0 },
		"long":        func(p *models.HookParams) { p.Timeout = models.MaxTimeout + time.Second },
		"no command":  func(p *models.HookParams) { p.Script.Command = " " },
		"env name":    func(p *models.HookParams) { p.Script.Env = []models.EnvVar{{Name: "A=B"}} },
		"empty agent": func(p *models.HookParams) { p.Kind = models.KindAgent },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := script()
			mutate(&p)
			_, err := models.NewHook(p)
			assert.True(t, errors.Is(err, hookerrors.ErrInvalidHook), "got %v", err)
		})
	}
}
