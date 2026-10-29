package pty

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScrollbackIsBoundedToTheLimit(t *testing.T) {
	output := newSharedOutput()

	chunk := []byte(strings.Repeat("x", 8192))
	for range (scrollbackLimit / 8192) + 5 {
		output.append(chunk)
	}

	scrollback, _, detach := output.subscribe()
	defer detach()
	require.Len(t, scrollback, scrollbackLimit)
}

func TestScrollbackKeepsTheNewestBytes(t *testing.T) {
	output := newSharedOutput()
	output.append(bytes.Repeat([]byte("a"), scrollbackLimit))
	output.append([]byte("NEWEST"))

	scrollback, _, detach := output.subscribe()
	defer detach()
	require.True(t, bytes.HasSuffix(scrollback, []byte("NEWEST")))
	require.Len(t, scrollback, scrollbackLimit)
}

func TestSubscriberReceivesLiveChunksAfterTheSnapshot(t *testing.T) {
	output := newSharedOutput()
	output.append([]byte("before"))

	scrollback, live, detach := output.subscribe()
	defer detach()
	output.append([]byte("after"))

	require.Equal(t, []byte("before"), scrollback)
	require.Equal(t, []byte("after"), <-live)
}

func TestSlowSubscriberLosesOldestChunksInsteadOfBlockingTheReader(t *testing.T) {
	output := newSharedOutput()
	_, live, detach := output.subscribe()
	defer detach()

	// Nobody reads yet: the subscriber holds at most subscriberCapacity chunks.
	for index := range subscriberCapacity + 10 {
		output.append([]byte{byte(index)})
	}

	require.Len(t, live, subscriberCapacity)
	first := <-live
	require.Equal(t, []byte{10}, first, "the oldest ten chunks are dropped, the newest are kept")
}

func TestFinishClosesLiveChannelsAfterDrainingBufferedChunks(t *testing.T) {
	output := newSharedOutput()
	_, live, detach := output.subscribe()
	defer detach()
	output.append([]byte("last"))

	output.finish()

	require.False(t, output.isRunning())
	require.Equal(t, []byte("last"), <-live)
	select {
	case _, open := <-live:
		require.False(t, open)
	case <-time.After(time.Second):
		t.Fatal("the live channel should be closed after finish")
	}
}

func TestSubscribingToAFinishedOutputYieldsAClosedChannel(t *testing.T) {
	output := newSharedOutput()
	output.append([]byte("old"))
	output.finish()

	scrollback, live, detach := output.subscribe()
	defer detach()

	require.Equal(t, []byte("old"), scrollback)
	_, open := <-live
	require.False(t, open)
}

func TestDetachStopsDeliveryAndIsIdempotent(t *testing.T) {
	output := newSharedOutput()
	_, live, detach := output.subscribe()

	detach()
	detach()
	output.append([]byte("unseen"))

	require.Empty(t, live)
}

func TestPumpCopiesReadsIntoTheOutputUntilEndOfFile(t *testing.T) {
	output := newSharedOutput()
	_, live, detach := output.subscribe()
	defer detach()

	pump(strings.NewReader("hello"), output)

	require.Equal(t, []byte("hello"), <-live)
	require.False(t, output.isRunning())
}

func TestCleanScrollbackStartPreventsBrokenAnsiSequences(t *testing.T) {
	// A line with an ANSI synchronized output sequence \x1b[?2026h followed by a newline and text.
	data := []byte("prefix\x1b[?2026h\nsecond line\nthird line")
	// If excess lands in the middle of \x1b[?2026h:
	excess := 8 // lands at '[' in \x1b[?2026h
	cut := cleanScrollbackStart(data, excess)
	result := string(data[cut:])
	require.False(t, strings.HasPrefix(result, "[?2026h"), "must not start with orphaned bracket sequence")
	require.False(t, strings.HasPrefix(result, "?2026h"), "must not start with orphaned question-mark sequence")
	require.Equal(t, "second line\nthird line", result)
}

func TestCleanScrollbackStartWithoutNewlineDiscardsPartialAnsi(t *testing.T) {
	// No newlines: raw escape sequence
	data := []byte("abc\x1b[?2026hxyz")
	// Excess lands on '?' (index 5)
	excess := 5
	cut := cleanScrollbackStart(data, excess)
	result := string(data[cut:])
	require.Equal(t, "xyz", result, "partial escape sequence must be discarded")
}

func TestCleanScrollbackStartPreservesUtf8Boundaries(t *testing.T) {
	// 🌸 is 4 bytes: F0 9F 8C B8
	emoji := "🌸"
	data := []byte("abc" + emoji + "def")
	// Excess lands on second byte of emoji (index 4)
	excess := 4
	cut := cleanScrollbackStart(data, excess)
	result := string(data[cut:])
	require.Equal(t, "def", result, "continuation bytes of UTF-8 must be skipped")
}
