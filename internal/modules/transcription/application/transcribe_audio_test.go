package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/transcription/application"
	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
	"iworkflow/internal/modules/transcription/domain/models"
)

func TestTranscribeAudioReturnsTheTrimmedText(t *testing.T) {
	recognizer := &fakeRecognizer{text: "  Hola mundo.\n"}
	useCase := application.NewTranscribeAudio(recognizer)

	transcript, err := useCase.Execute(context.Background(), application.TranscribeInput{WAV: validWAV()})

	require.NoError(t, err)
	require.Equal(t, "Hola mundo.", transcript.Text)
	require.Equal(t, 1, recognizer.calls)
	require.Equal(t, models.SampleRate, recognizer.rate)
	require.Len(t, recognizer.samples, 1600)
}

func TestTranscribeAudioFailures(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name       string
		input      []byte
		recognizer *fakeRecognizer
		want       error
		wantCalls  int
	}{
		{"invalid audio never reaches the recognizer", []byte("nope"), &fakeRecognizer{}, transcriptionerrors.ErrInvalidAudio, 0},
		{"a blank text is no speech", validWAV(), &fakeRecognizer{text: " \n "}, transcriptionerrors.ErrNoSpeech, 1},
		{"the recognizer failure is wrapped", validWAV(), &fakeRecognizer{err: boom}, boom, 1},
		{"an unavailable recognizer is kept", validWAV(), &fakeRecognizer{err: transcriptionerrors.ErrUnavailable}, transcriptionerrors.ErrUnavailable, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useCase := application.NewTranscribeAudio(tt.recognizer)

			_, err := useCase.Execute(context.Background(), application.TranscribeInput{WAV: tt.input})

			require.ErrorIs(t, err, tt.want)
			require.Equal(t, tt.wantCalls, tt.recognizer.calls)
		})
	}
}

func TestTranscribeAudioHonoursACancelledContext(t *testing.T) {
	recognizer := &fakeRecognizer{text: "x"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := application.NewTranscribeAudio(recognizer).Execute(ctx, application.TranscribeInput{WAV: validWAV()})

	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, recognizer.calls)
}
