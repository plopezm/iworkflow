package sherpa_test

import (
	"context"
	"testing"

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
