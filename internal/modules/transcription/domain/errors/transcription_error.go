// Package errors holds the business errors of the transcription module. The messages are for logs; the
// inbound adapter maps each sentinel to a translated text and an HTTP status.
package errors

import "errors"

var (
	// ErrInvalidAudio means the body is not mono 16 kHz 16-bit PCM WAV audio, or has no samples.
	ErrInvalidAudio = errors.New("invalid audio: expected mono 16 kHz 16-bit PCM WAV with samples")
	// ErrAudioTooLong means the recording is longer than the supported duration.
	ErrAudioTooLong = errors.New("audio is too long")
	// ErrNoSpeech means the recognizer produced no text.
	ErrNoSpeech = errors.New("no speech recognized")
	// ErrUnavailable means transcription cannot run here: the model is missing or the build has no recognizer.
	ErrUnavailable = errors.New("transcription is unavailable")
)
