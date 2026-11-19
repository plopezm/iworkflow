//go:build unix && !linux

package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersOfReportsATimedOutLsofEvenWithPartialOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := usersOf(ctx, []string{t.TempDir()})

	assert.Error(t, err)
}

func TestUsersOfGivesTheSameStartTokenToTheSameProcessAcrossScans(t *testing.T) {
	root := t.TempDir()
	start(t, root, "sleep", "60")

	first, err := usersOf(context.Background(), rootsOf(root))
	require.NoError(t, err)
	second, err := usersOf(context.Background(), rootsOf(root))
	require.NoError(t, err)

	require.Len(t, first, 1)
	require.Len(t, second, 1)
	assert.NotEmpty(t, first[0].Started)
	assert.Equal(t, first[0].key(), second[0].key())
}
