package state

import "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"

// FinishPreviewView is the dialog that asks before a finish: the confirmation and, when processes use the worktree,
// one checkbox per process. Scope is the page instance the dialog answers to.
type FinishPreviewView struct {
	Scope     string
	TaskID    string
	Processes []dto.FinishProcessDTO
}

// FinishPreviewPath is the address of the finish dialog of a task, with the page scope when it is set.
func FinishPreviewPath(taskID string, scope string) string {
	return ScopedPath(TaskPath(taskID)+"/finish/preview", scope)
}

// FinishActionPath is the address the finish dialog posts the finish to, with the page scope when it is set.
func FinishActionPath(taskID string, scope string) string {
	return ScopedPath(TaskPath(taskID)+"/finish", scope)
}

// FinishSlotID is the id of the slot the finish dialog is loaded into. Each page instance has its own slot.
func FinishSlotID(scope string) string {
	return Scoped("task-finish", scope)
}
