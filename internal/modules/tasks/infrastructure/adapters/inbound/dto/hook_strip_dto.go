package dto

import (
	"time"

	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// HookStripDTO is one hook of the status strip of a task, as the presentation shows it.
type HookStripDTO struct {
	HookID        string
	Kind          string
	Description   string
	Phase         string
	Event         string
	Enabled       bool
	Skipped       bool
	LastStatus    string
	LastOutput    string
	LastExitCode  int
	LastStartedAt time.Time
}

// HookStripFromPort maps a hook of the strip port to its DTO.
func HookStripFromPort(hook outbound.StripHook) HookStripDTO {
	return HookStripDTO{
		HookID:        hook.HookID,
		Kind:          hook.Kind,
		Description:   hook.Description,
		Phase:         hook.Phase,
		Event:         hook.Event,
		Enabled:       hook.Enabled,
		Skipped:       hook.Skipped,
		LastStatus:    hook.LastStatus,
		LastOutput:    hook.LastOutput,
		LastExitCode:  hook.LastExitCode,
		LastStartedAt: hook.LastStartedAt,
	}
}
