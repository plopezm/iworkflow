// Package application holds the use cases of the transcription module.
package application

import (
	"context"
	"fmt"
	"strings"

	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
	"iworkflow/internal/modules/transcription/domain/models"
	"iworkflow/internal/modules/transcription/domain/ports/outbound"
)

// TranscribeInput is a recording as the client sent it.
type TranscribeInput struct {
	// WAV is a mono 16 kHz 16-bit PCM WAV file.
	WAV []byte
}

// TranscribeAudio validates a recording and turns it into text.
type TranscribeAudio struct {
	recognizer outbound.Recognizer
}

// NewTranscribeAudio builds the use case.
func NewTranscribeAudio(recognizer outbound.Recognizer) *TranscribeAudio {
	return &TranscribeAudio{recognizer: recognizer}
}

// Execute decodes the WAV and runs the recognizer. A blank result is ErrNoSpeech.
func (u *TranscribeAudio) Execute(ctx context.Context, input TranscribeInput) (models.Transcript, error) {
	if err := ctx.Err(); err != nil {
		return models.Transcript{}, fmt.Errorf("transcribe audio: %w", err)
	}
	audio, err := models.ParseWAV(input.WAV)
	if err != nil {
		return models.Transcript{}, fmt.Errorf("read audio: %w", err)
	}
	text, err := u.recognizer.Recognize(ctx, audio.Samples(), audio.SampleRate())
	if err != nil {
		return models.Transcript{}, fmt.Errorf("recognize speech: %w", err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return models.Transcript{}, transcriptionerrors.ErrNoSpeech
	}
	return models.Transcript{Text: text}, nil
}
