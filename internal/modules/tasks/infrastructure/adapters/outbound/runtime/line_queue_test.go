package runtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rule from source: agent_process.rs uses an unbounded mpsc channel for stdin lines; lines come out in
// the order they went in, which the protocols depend on (handshake before prompt).
func TestLineQueuePopReturnsLinesInFIFOOrder(t *testing.T) {
	queue := newLineQueue()
	require.True(t, queue.push("one"))
	require.True(t, queue.push("two"))
	require.True(t, queue.push("three"))

	for _, want := range []string{"one", "two", "three"} {
		line, ok := queue.pop()
		assert.True(t, ok)
		assert.Equal(t, want, line)
	}
}

// Rule from source: the writer waits for the next command instead of spinning or ending early.
func TestLineQueuePopBlocksUntilALinePushed(t *testing.T) {
	queue := newLineQueue()
	popped := make(chan string, 1)
	go func() {
		line, _ := queue.pop()
		popped <- line
	}()

	select {
	case line := <-popped:
		require.FailNowf(t, "pop returned before any push", "got %q", line)
	case <-time.After(50 * time.Millisecond):
	}
	require.True(t, queue.push("late"))

	select {
	case line := <-popped:
		assert.Equal(t, "late", line)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "pop did not wake up after push")
	}
}

// Rule from source: closing the channel (stop drops the sender) lets the writer drain what is already
// queued, then end. Mirrors mpsc recv returning buffered messages before None.
func TestLineQueueCloseKeepsBufferedLinesThenEnds(t *testing.T) {
	queue := newLineQueue()
	require.True(t, queue.push("buffered"))

	queue.close()

	line, ok := queue.pop()
	assert.True(t, ok)
	assert.Equal(t, "buffered", line)
	_, ok = queue.pop()
	assert.False(t, ok)
}

// Rule from source: after the writer has gone, send must fail with AgentNotRunning, so a push onto a
// closed queue is rejected instead of silently dropped.
func TestLineQueuePushAfterCloseIsRejected(t *testing.T) {
	queue := newLineQueue()
	queue.close()

	assert.False(t, queue.push("too late"))
}
