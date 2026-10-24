// Package inbound holds the contracts the tasks module exposes to its drivers (UI, runtimes, background loops).
package inbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// AgentEventSink receives the events an agent emits while it runs. The runtime calls it sequentially,
// in arrival order, for each task. Failures are logged by the receiver, never returned to the runtime.
type AgentEventSink interface {
	Handle(ctx context.Context, taskID models.TaskID, event models.AgentEvent)
}

// StartTaskUseCase starts a task in backlog. The UI and the graph trigger consume it.
type StartTaskUseCase interface {
	Start(ctx context.Context, id models.TaskID) (models.Task, error)
}

// SuccessorTrigger evaluates the graph triggers and starts the tasks that can start now.
type SuccessorTrigger interface {
	// Trigger returns how many tasks were started.
	Trigger(ctx context.Context) (int, error)
}
