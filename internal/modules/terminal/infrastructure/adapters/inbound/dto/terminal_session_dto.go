// Package dto holds the terminal payloads that cross the client/server boundary. They never expose
// the domain types or any persistence struct.
package dto

import (
	"time"

	"iworkflow/internal/modules/terminal/domain/models"
)

// TerminalStatusDTO is the wire form of the session status. Its values match the Rust serde names.
type TerminalStatusDTO string

const (
	// TerminalStatusRunningDTO is sent while the shell is alive.
	TerminalStatusRunningDTO TerminalStatusDTO = "Running"
	// TerminalStatusExitedDTO is sent once the shell has ended.
	TerminalStatusExitedDTO TerminalStatusDTO = "Exited"
)

// TerminalSessionDTO is the JSON form of a terminal session.
type TerminalSessionDTO struct {
	ID            string            `json:"id"`
	WorkspaceID   string            `json:"workspace_id"`
	WorkspaceName string            `json:"workspace_name"`
	CreatedAt     string            `json:"created_at"`
	Status        TerminalStatusDTO `json:"status"`
}

// FromTerminalSession maps a domain session to its DTO. created_at uses the RFC 3339 form that chrono
// produces: UTC offset "+00:00", and fractional seconds only when they are not zero.
func FromTerminalSession(session models.TerminalSession) TerminalSessionDTO {
	return TerminalSessionDTO{
		ID:            session.ID.String(),
		WorkspaceID:   session.WorkspaceID.String(),
		WorkspaceName: session.WorkspaceName,
		CreatedAt:     formatChronoRFC3339(session.CreatedAt),
		Status:        statusToDTO(session.Status),
	}
}

// statusToDTO maps the domain status to its wire value.
func statusToDTO(status models.TerminalStatus) TerminalStatusDTO {
	if status == models.TerminalStatusExited {
		return TerminalStatusExitedDTO
	}
	return TerminalStatusRunningDTO
}

// formatChronoRFC3339 renders t like chrono's to_rfc3339 for UTC: fractional digits in steps of three
// (none, millis, micros or nanos) and the "+00:00" offset.
func formatChronoRFC3339(t time.Time) string {
	utc := t.UTC()
	layout := "2006-01-02T15:04:05"
	switch nanos := utc.Nanosecond(); {
	case nanos == 0:
	case nanos%int(time.Millisecond) == 0:
		layout += ".000"
	case nanos%int(time.Microsecond) == 0:
		layout += ".000000"
	default:
		layout += ".000000000"
	}
	return utc.Format(layout) + "+00:00"
}
