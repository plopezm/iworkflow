package outbound

import "iworkflow/internal/modules/tasks/domain/models"

// AgentProcessLocator tells which OS process runs the live agent of a task, so the finish flow can leave the
// task's own agent out of the list of processes it offers to close: that agent is stopped without asking.
type AgentProcessLocator interface {
	// AgentPID returns the pid of the live agent of taskID. It is false when the task has no live agent process.
	AgentPID(taskID models.TaskID) (int, bool)
}
