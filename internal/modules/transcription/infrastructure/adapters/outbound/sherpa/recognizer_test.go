package sherpa_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
	"iworkflow/internal/modules/transcription/infrastructure/adapters/outbound/sherpa"
)

func TestRecognizeWithoutModelIsUnavailable(t *testing.T) {
	recognizer := sherpa.NewRecognizer(t.TempDir())
	defer recognizer.Close()

	_, err := recognizer.Recognize(context.Background(), []float32{0.1}, 16000)

	require.ErrorIs(t, err, transcriptionerrors.ErrUnavailable)
}

func TestRecognizeHonoursACancelledContext(t *testing.T) {
	recognizer := sherpa.NewRecognizer(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := recognizer.Recognize(ctx, []float32{0.1}, 16000)

	require.ErrorIs(t, err, context.Canceled)
}

func TestRecognizeAfterCloseIsUnavailable(t *testing.T) {
	recognizer := sherpa.NewRecognizer(t.TempDir())
	recognizer.Close()

	_, err := recognizer.Recognize(context.Background(), []float32{0.1}, 16000)

	require.ErrorIs(t, err, transcriptionerrors.ErrUnavailable)
}

func TestRecognizeStopsWaitingWhenTheContextEnds(t *testing.T) {
	recognizer := sherpa.NewRecognizer(t.TempDir())
	release := recognizer.HoldForTest()
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := recognizer.Recognize(ctx, []float32{0.1}, 16000)
		done <- err
	}()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(2 * time.Second):
		t.Fatal("Recognize kept waiting for the busy recognizer after its context ended")
	}
}
