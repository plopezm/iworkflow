package components

import (
	"fmt"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// GhostStyle returns the CSS style attribute string for the drag ghost element following the pointer.
func GhostStyle(pointer state.Point) string {
	return fmt.Sprintf("left: %.0fpx; top: %.0fpx;", pointer.X, pointer.Y)
}

// LooseCardKey returns a unique cache key for a loose task card reflecting its rendered fields.
func LooseCardKey(task dto.TaskDTO) string {
	errStr := ""
	if task.Error != nil {
		errStr = *task.Error
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%t",
		task.ID,
		task.Title,
		task.Stage,
		task.Status,
		task.Priority,
		errStr,
		task.PendingDialogs,
		task.NeedsAttention,
	)
}
