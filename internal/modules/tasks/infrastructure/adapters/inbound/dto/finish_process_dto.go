package dto

import (
	"fmt"
	"strconv"
	"strings"

	"iworkflow/internal/modules/tasks/domain/models"
)

// FinishProcessDTO is a process the finish dialog lists. Value is what its checkbox posts: the pid and the start token
// that identify the process.
type FinishProcessDTO struct {
	PID    int
	Name   string
	Path   string
	Reason string
	Value  string
}

// FinishProcessFromDomain maps a process the finish lists to its row of the dialog.
func FinishProcessFromDomain(process models.WorktreeProcess) FinishProcessDTO {
	return FinishProcessDTO{
		PID:    process.PID,
		Name:   process.Name,
		Path:   process.Path,
		Reason: process.Reason,
		Value:  FinishProcessValue(process),
	}
}

// FinishProcessValue encodes the identity of a process for a form field: "<pid>|<started>". The start token may hold
// spaces, so the pid is the part before the first bar.
func FinishProcessValue(process models.WorktreeProcess) string {
	return strconv.Itoa(process.PID) + "|" + process.Started
}

// DecodeFinishSelection reads the process fields a finish form posts. No value selects no process. A value without
// its bar or with a pid that is not a positive number is refused, and so is a pid chosen twice. An empty start token
// is accepted: a platform that cannot read the start time must not block the finish, and the identity then rests on
// the pid alone. The whole selection is refused on any of those: so nothing is closed or finished on a partial reading.
func DecodeFinishSelection(values []string) ([]models.WorktreeProcess, error) {
	if len(values) == 0 {
		return nil, nil
	}
	selected := make([]models.WorktreeProcess, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		pidText, started, found := strings.Cut(value, "|")
		if !found {
			return nil, fmt.Errorf("selección de proceso mal formada: %q", value)
		}
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("pid de proceso no válido: %q", value)
		}
		if _, dup := seen[pid]; dup {
			return nil, fmt.Errorf("el proceso %d está seleccionado dos veces", pid)
		}
		seen[pid] = struct{}{}
		selected = append(selected, models.WorktreeProcess{PID: pid, Started: started})
	}
	return selected, nil
}
