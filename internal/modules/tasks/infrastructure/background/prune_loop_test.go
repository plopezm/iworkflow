package background_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/infrastructure/background"
)

type countingPruner struct{ calls atomic.Int64 }

func (p *countingPruner) Execute(context.Context, time.Time) (int, error) {
	p.calls.Add(1)
	return 0, nil
}

func TestRunPruneLoopPrunesAtOnceAndAgainEveryInterval(t *testing.T) {
	pruner := &countingPruner{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- background.RunPruneLoop(ctx, pruner, time.Now, 5*time.Millisecond) }()

	require.Eventually(t, func() bool { return pruner.calls.Load() >= 3 }, 2*time.Second, time.Millisecond)
	cancel()
	assert.NoError(t, <-done)
}

func TestRunPruneLoopRejectsANonPositiveInterval(t *testing.T) {
	err := background.RunPruneLoop(context.Background(), &countingPruner{}, time.Now, 0)
	assert.ErrorIs(t, err, background.ErrNonPositiveInterval)
}
