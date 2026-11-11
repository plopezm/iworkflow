package models

import (
	"fmt"
	"strings"
)

// AssignmentParams are the raw fields NewAssignment validates.
type AssignmentParams struct {
	ID          string
	HookID      string
	WorkspaceID string
	Event       Event
	Phase       Phase
	Enabled     bool
	OnFailure   FailurePolicy
	Position    int
}

// Assignment attaches a library hook to the event and phase of one workspace, with its own enabled flag, failure
// policy and position among the hooks of that event and phase.
type Assignment struct {
	id          string
	hookID      string
	workspaceID string
	event       Event
	phase       Phase
	enabled     bool
	onFailure   FailurePolicy
	position    int
}

// NewAssignment validates the parameters and builds the assignment.
func NewAssignment(p AssignmentParams) (Assignment, error) {
	switch {
	case strings.TrimSpace(p.ID) == "":
		return Assignment{}, invalid("the assignment id is required")
	case strings.TrimSpace(p.HookID) == "":
		return Assignment{}, invalid("the hook is required")
	case strings.TrimSpace(p.WorkspaceID) == "":
		return Assignment{}, invalid("the project is required")
	case p.Phase != PhasePre && p.Phase != PhasePost:
		return Assignment{}, invalid(fmt.Sprintf("%q is not a valid phase", p.Phase))
	case p.Event != EventTaskStart && p.Event != EventTaskFinish:
		return Assignment{}, invalid(fmt.Sprintf("%q is not a valid event", p.Event))
	case p.OnFailure != FailureBlock && p.OnFailure != FailureWarn && p.OnFailure != FailureIgnore:
		return Assignment{}, invalid(fmt.Sprintf("%q is not a valid failure policy", p.OnFailure))
	}
	return Assignment{
		id: p.ID, hookID: p.HookID, workspaceID: p.WorkspaceID, event: p.Event, phase: p.Phase,
		enabled: p.Enabled, onFailure: p.OnFailure, position: p.Position,
	}, nil
}

// ID is the stable identifier of the assignment.
func (a Assignment) ID() string { return a.id }

// HookID is the library hook it runs.
func (a Assignment) HookID() string { return a.hookID }

// WorkspaceID is the project the hook runs for.
func (a Assignment) WorkspaceID() string { return a.workspaceID }

// Event is the lifecycle point.
func (a Assignment) Event() Event { return a.event }

// Phase is pre or post.
func (a Assignment) Phase() Phase { return a.phase }

// Enabled says whether the assignment runs at all.
func (a Assignment) Enabled() bool { return a.enabled }

// OnFailure is the failure policy.
func (a Assignment) OnFailure() FailurePolicy { return a.onFailure }

// Position orders the assignments of one workspace, event and phase.
func (a Assignment) Position() int { return a.position }

// Params returns the fields the assignment was built from, so a change can be validated again.
func (a Assignment) Params() AssignmentParams {
	return AssignmentParams{
		ID: a.id, HookID: a.hookID, WorkspaceID: a.workspaceID, Event: a.event, Phase: a.phase,
		Enabled: a.enabled, OnFailure: a.onFailure, Position: a.position,
	}
}

// AssignedHook is an assignment with the hook it runs.
type AssignedHook struct {
	Assignment Assignment
	Hook       Hook
}
