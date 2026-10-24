package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// waitTimeout bounds every wait on the outside world (subprocess output, channels) so a broken build fails
// instead of hanging the suite.
const waitTimeout = 10 * time.Second

func newTaskID(t *testing.T) models.TaskID {
	t.Helper()
	taskID, err := models.NewTaskID()
	require.NoError(t, err)
	return taskID
}

// recordedEvent is one event delivered to a fakeEventSink.
type recordedEvent struct {
	taskID models.TaskID
	event  models.AgentEvent
}

// fakeEventSink records every event it receives, in arrival order.
type fakeEventSink struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (s *fakeEventSink) Handle(_ context.Context, taskID models.TaskID, event models.AgentEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, recordedEvent{taskID: taskID, event: event})
}
