package inbound

import (
	"context"
	"time"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// memoryDateLayout is the date each memory of the Memory panel shows.
const memoryDateLayout = "2006-01-02"

// memoryPanelWait is the longest the task detail page waits for Engram. Past it the panel is left out and the page
// renders without it.
const memoryPanelWait = time.Second

// TaskMemoryLister reads the Memory panel of a task. *application.ListTaskMemory satisfies it. It is optional: when it is
// nil the detail page shows no Memory panel.
type TaskMemoryLister interface {
	Execute(ctx context.Context, input application.TaskMemoryInput) (models.TaskMemoryFeed, error)
}

// memoryPanel builds the Memory panel of a task. It is nil, so the panel is absent, when no use case is wired, when
// Engram is not available, when the project has no memories, or when the read fails (the failure is logged). The page
// renders in every case.
func (h *DetailHandlers) memoryPanel(ctx context.Context, scope string, task models.Task) *state.MemoryPanel {
	if h.uc.Memory == nil {
		return nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, memoryPanelWait)
	defer cancel()
	feed, err := h.uc.Memory.Execute(waitCtx, application.TaskMemoryInput{WorkspaceID: task.WorkspaceID})
	if err != nil {
		web.LogError(ctx, "could not read the memories of the task", err, "task_id", task.ID.String())
		return nil
	}
	if !feed.Available || len(feed.Memories) == 0 {
		return nil
	}
	rows := make([]state.MemoryRow, 0, len(feed.Memories))
	for _, memory := range feed.Memories {
		rows = append(rows, state.MemoryRow{
			Type:  memory.Type,
			Title: memory.Title,
			Date:  memory.LastSeenAt.Format(memoryDateLayout),
		})
	}
	return &state.MemoryPanel{Scope: scope, Rows: rows}
}
