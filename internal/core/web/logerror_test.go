package web

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func TestLogError(t *testing.T) {
	failure := errors.New("boom")

	t.Run("live request is logged as error", func(t *testing.T) {
		buf := captureLog(t)
		LogError(context.Background(), "could not load", failure)
		assert.Contains(t, buf.String(), "level=ERROR")
		assert.Contains(t, buf.String(), "could not load")
	})

	t.Run("canceled request is logged as debug", func(t *testing.T) {
		buf := captureLog(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		LogError(ctx, "could not load", failure)
		assert.NotContains(t, buf.String(), "level=ERROR")
		assert.Contains(t, buf.String(), "level=DEBUG")
	})
}
