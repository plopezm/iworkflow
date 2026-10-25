package state

import "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"

// TaskDetailView is what the detail page of one task shows. Scope is the page instance: its element ids and its
// action targets carry it, so two tabs of the same task never share an id in one document.
type TaskDetailView struct {
	Scope string
	Task  dto.TaskDTO
}
