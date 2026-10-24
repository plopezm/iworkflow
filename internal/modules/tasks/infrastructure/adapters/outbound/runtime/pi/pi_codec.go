package pi

import (
	"slices"
	"sync"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime"
)

// subagentStore keeps the last known snapshot of the subagents of each task. A snapshot outlives the agent
// process; only a new start of the task discards it. Access is by task id only, so no iteration order matters.
type subagentStore struct {
	mu        sync.Mutex
	snapshots map[models.TaskID][]models.SubagentTask
}

func newSubagentStore() *subagentStore {
	return &subagentStore{snapshots: make(map[models.TaskID][]models.SubagentTask)}
}

func (s *subagentStore) set(taskID models.TaskID, tasks []models.SubagentTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[taskID] = tasks
}

func (s *subagentStore) remove(taskID models.TaskID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, taskID)
}

func (s *subagentStore) get(taskID models.TaskID) []models.SubagentTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.snapshots[taskID])
}

// piCodec is the protocol of one pi process: it decodes its stdout, encodes the commands and keeps the
// subagent snapshot and the activity counters of the task. ProcessRuntime serializes its calls, so it needs
// no locking of its own. It has no handshake, so it embeds NoHandshake.
type piCodec struct {
	runtime.NoHandshake
	subagents *subagentStore
	activity  activityTracker
}

var _ runtime.LineCodec = (*piCodec)(nil)

func newPiCodec(subagents *subagentStore) *piCodec {
	return &piCodec{subagents: subagents}
}

// Decode records the subagent snapshot the line carries, before the events are returned, so a consumer
// that reacts to SubagentsChanged already finds the snapshot. The events get the activity counters folded in.
func (c *piCodec) Decode(taskID models.TaskID, line string) []models.AgentEvent {
	if tasks, ok := decodeSubagentTasks(line); ok {
		c.subagents.set(taskID, tasks)
	}
	return c.activity.fold(decode(line))
}

// Encode returns the stdin record of a command, without its line ending.
func (c *piCodec) Encode(command models.AgentCommand, requestID string) (string, bool, error) {
	return encodeCommand(command, requestID)
}
