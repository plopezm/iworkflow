package state

import (
	"strings"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

// BoardMode is how the board groups the imported issues.
type BoardMode string

const (
	// BoardModeEpics groups the issues by Epic (the default).
	BoardModeEpics BoardMode = "epics"
	// BoardModeLanes groups the issues by the JQL lane that imported them.
	BoardModeLanes BoardMode = "lanes"
)

// ParseBoardMode reads the mode of the query string; anything unknown is the default.
func ParseBoardMode(raw string) BoardMode {
	if BoardMode(strings.TrimSpace(raw)) == BoardModeLanes {
		return BoardModeLanes
	}
	return BoardModeEpics
}

// EpicTitle is the heading of an epic group: its summary, else its key.
func EpicTitle(group dto.EpicGroupDTO) string {
	if group.Epic != nil && group.Epic.Summary != "" {
		return group.Epic.Summary
	}
	return group.Key
}

// IsNoEpic reports whether the group holds the issues without an Epic.
func IsNoEpic(group dto.EpicGroupDTO) bool {
	return group.Key == models.NoEpicKey
}

// ProgressPercent is the share of finished children, 0 when there are none.
func ProgressPercent(group dto.EpicGroupDTO) int {
	if len(group.Children) == 0 {
		return 0
	}
	return group.Done * 100 / len(group.Children)
}

// StatusVariant picks the badge variant of a Jira status from its name.
func StatusVariant(status string) string {
	name := strings.ToLower(strings.TrimSpace(status))
	switch {
	case models.IsDoneStatus(name):
		return "done"
	case containsAny(name, "block", "bloque", "impedi"):
		return "attention"
	case containsAny(name, "review", "revis", "test", "prueba", "qa"):
		return "review"
	case containsAny(name, "progress", "curso", "doing", "desarroll", "working"):
		return "working"
	default:
		return "backlog"
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

// StatusMenuView is the list of transitions offered for one child issue of an epic.
type StatusMenuView struct {
	EpicKey     string
	IssueKey    string
	Status      string
	Transitions []dto.TransitionDTO
	Err         string
}
