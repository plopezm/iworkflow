package system_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/system"
)

func TestClockReturnsTheCurrentTime(t *testing.T) {
	t.Parallel()
	before := time.Now()

	got := system.NewClock().Now()

	assert.False(t, got.Before(before), "the clock does not run backwards")
	assert.WithinDuration(t, time.Now(), got, time.Minute)
}

func TestUUIDGeneratorReturnsDistinctVersion4UUIDs(t *testing.T) {
	t.Parallel()
	generator := system.NewUUIDGenerator()

	first := generator.NewID()
	second := generator.NewID()

	require.NotEqual(t, first, second)
	parsed, err := uuid.Parse(first)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(4), parsed.Version())
}
