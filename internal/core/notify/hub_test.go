package notify_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/notify"
)

func TestHubDeliversToEverySubscriber(t *testing.T) {
	hub := notify.NewHub()
	first, cancelFirst := hub.Subscribe()
	defer cancelFirst()
	second, cancelSecond := hub.Subscribe()
	defer cancelSecond()

	hub.Publish("agent-settings")

	require.Len(t, first.Ready(), 1)
	require.Equal(t, []string{"agent-settings"}, first.Drain())
	require.Equal(t, []string{"agent-settings"}, second.Drain())
	require.Empty(t, first.Drain(), "drain clears the pending notices")
}

func TestHubStopsDeliveringAfterCancel(t *testing.T) {
	hub := notify.NewHub()
	sub, cancel := hub.Subscribe()
	cancel()

	hub.Publish("x")

	require.Empty(t, sub.Drain())
}

func TestHubCoalescesRepeatedNoticesAndKeepsEveryDistinctName(t *testing.T) {
	hub := notify.NewHub()
	sub, cancel := hub.Subscribe()
	defer cancel()

	for range 100 {
		hub.Publish("agent-settings")
	}
	hub.Publish("other")

	require.Equal(t, []string{"agent-settings", "other"}, sub.Drain())
}

func TestHubDoesNotBlockWhenNobodyDrains(t *testing.T) {
	hub := notify.NewHub()
	_, cancel := hub.Subscribe()
	defer cancel()

	for range 1000 {
		hub.Publish("x")
	}
}

func TestNilHubIsInert(t *testing.T) {
	var hub *notify.Hub
	hub.Publish("x")
	sub, cancel := hub.Subscribe()
	cancel()
	require.Empty(t, sub.Drain())
}
