package application_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/domain/models"
)

// libraryHook builds a valid hook of the library of the given kind.
func libraryHook(t *testing.T, id string, kind models.Kind) models.Hook {
	t.Helper()
	params := models.HookParams{ID: id, Name: "hook " + id, Kind: kind, Timeout: time.Minute}
	if kind == models.KindScript {
		params.Script = models.ScriptSpec{Command: "echo", Args: []string{"hello"}}
	} else {
		params.Agent = models.AgentSpec{Prompt: "check the branch"}
	}
	hook, err := models.NewHook(params)
	require.NoError(t, err)
	return hook
}

// assignment builds a valid assignment with the id "as-<hookID>".
func assignment(t *testing.T, hookID string, workspaceID string, event models.Event, phase models.Phase, policy models.FailurePolicy, enabled bool) models.Assignment {
	t.Helper()
	a, err := models.NewAssignment(models.AssignmentParams{
		ID: "as-" + hookID, HookID: hookID, WorkspaceID: workspaceID, Event: event, Phase: phase,
		Enabled: enabled, OnFailure: policy,
	})
	require.NoError(t, err)
	return a
}
