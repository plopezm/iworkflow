package models_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
)

func assignmentParams() models.AssignmentParams {
	return models.AssignmentParams{
		ID: "a1", HookID: "h1", WorkspaceID: "w1", Event: models.EventTaskStart, Phase: models.PhasePre,
		Enabled: true, OnFailure: models.FailureBlock, Position: 3,
	}
}

func TestNewAssignmentValid(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*models.AssignmentParams){
		"task start pre block": func(p *models.AssignmentParams) {},
		"task finish post warn": func(p *models.AssignmentParams) {
			p.Event, p.Phase, p.OnFailure = models.EventTaskFinish, models.PhasePost, models.FailureWarn
		},
		"disabled ignore policy": func(p *models.AssignmentParams) { p.Enabled, p.OnFailure = false, models.FailureIgnore },
		"first position":         func(p *models.AssignmentParams) { p.Position = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := assignmentParams()
			mutate(&p)

			assignment, err := models.NewAssignment(p)

			require.NoError(t, err)
			assert.Equal(t, p.ID, assignment.ID())
			assert.Equal(t, p.HookID, assignment.HookID())
			assert.Equal(t, p.WorkspaceID, assignment.WorkspaceID())
			assert.Equal(t, p.Event, assignment.Event())
			assert.Equal(t, p.Phase, assignment.Phase())
			assert.Equal(t, p.Enabled, assignment.Enabled())
			assert.Equal(t, p.OnFailure, assignment.OnFailure())
			assert.Equal(t, p.Position, assignment.Position())
		})
	}
}

func TestNewAssignmentInvalid(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*models.AssignmentParams){
		"blank id":        func(p *models.AssignmentParams) { p.ID = " " },
		"blank hook":      func(p *models.AssignmentParams) { p.HookID = "" },
		"blank workspace": func(p *models.AssignmentParams) { p.WorkspaceID = "\t" },
		"unknown phase":   func(p *models.AssignmentParams) { p.Phase = "during" },
		"empty phase":     func(p *models.AssignmentParams) { p.Phase = "" },
		"unknown event":   func(p *models.AssignmentParams) { p.Event = "task_discard" },
		"empty event":     func(p *models.AssignmentParams) { p.Event = "" },
		"unknown policy":  func(p *models.AssignmentParams) { p.OnFailure = "retry" },
		"empty policy":    func(p *models.AssignmentParams) { p.OnFailure = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := assignmentParams()
			mutate(&p)

			_, err := models.NewAssignment(p)

			assert.True(t, errors.Is(err, hookerrors.ErrInvalidHook), "got %v", err)
		})
	}
}

func TestAssignmentParamsRoundTrip(t *testing.T) {
	t.Parallel()
	assignment, err := models.NewAssignment(assignmentParams())
	require.NoError(t, err)

	again, err := models.NewAssignment(assignment.Params())

	require.NoError(t, err)
	assert.Equal(t, assignment, again)
}
