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

	require.Equal(t, "agent-settings", <-first)
	require.Equal(t, "agent-settings", <-second)
}

func TestHubStopsDeliveringAfterCancel(t *testing.T) {
	hub := notify.NewHub()
	sub, cancel := hub.Subscribe()
	cancel()

	hub.Publish("x")

	require.Empty(t, sub)
}

func TestHubDropsNoticesOfASlowSubscriberWithoutBlocking(t *testing.T) {
	hub := notify.NewHub()
	sub, cancel := hub.Subscribe()
	defer cancel()

	for range 100 {
		hub.Publish("x")
	}

	require.NotEmpty(t, sub)
}

func TestNilHubIsInert(t *testing.T) {
	var hub *notify.Hub
	hub.Publish("x")
	_, cancel := hub.Subscribe()
	cancel()
}
