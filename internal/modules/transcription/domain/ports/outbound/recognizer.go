// Package outbound holds the ports the transcription use cases need from the infrastructure.
package outbound

import "context"

// Recognizer turns speech into text.
type Recognizer interface {
	// Recognize transcribes mono samples in [-1, 1] recorded at sampleRate. It returns ErrUnavailable
	// (domain errors) when the recognizer cannot run, and honours the cancellation of ctx.
	Recognize(ctx context.Context, samples []float32, sampleRate int) (string, error)
}
