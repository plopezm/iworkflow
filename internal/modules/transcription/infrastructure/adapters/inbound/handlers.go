// Package inbound is the HTTP adapter of the transcription module: it takes a recording and answers its text.
package inbound

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/modules/transcription/application"
	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
	"iworkflow/internal/modules/transcription/domain/models"
	"iworkflow/internal/modules/transcription/infrastructure/adapters/inbound/dto"
)

const (
	// TranscriptionsPath is the endpoint the recorder posts the WAV to.
	TranscriptionsPath = "/transcriptions"
	// MaxBodyBytes is the largest body read: MaxSeconds of 16 kHz 16-bit mono audio plus the WAV header and slack.
	retryAfterSeconds = "5"
	MaxBodyBytes      = models.MaxSeconds*models.SampleRate*2 + 4096
)

// transcriber is the use case the handler consumes.
type transcriber interface {
	Execute(ctx context.Context, input application.TranscribeInput) (models.Transcript, error)
}

// Handlers serves the transcription endpoint.
type Handlers struct {
	transcribe transcriber
	logger     *slog.Logger
	// running admits one transcription at a time: the model uses every thread it is given, so a queue of waiting
	// requests would only hold memory and workers while the CPU is saturated. The others are told to come back.
	running chan struct{}
}

// NewHandlers builds the handlers. The logger records the failures the user is not told about; nil logs nothing.
func NewHandlers(transcribe transcriber, logger ...*slog.Logger) *Handlers {
	handlers := &Handlers{transcribe: transcribe, logger: slog.New(slog.DiscardHandler), running: make(chan struct{}, 1)}
	if len(logger) > 0 && logger[0] != nil {
		handlers.logger = logger[0]
	}
	return handlers
}

// Routes mounts the endpoint.
func (h *Handlers) Routes(r chi.Router) {
	r.Post(TranscriptionsPath, h.post)
}

// post reads the WAV body, transcribes it and answers {"text": ...} or {"error": ...}.
func (h *Handlers) post(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(MaxBodyBytes))
	wav, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			err = transcriptionerrors.ErrAudioTooLong
		} else {
			err = transcriptionerrors.ErrInvalidAudio
		}
		h.fail(w, r, err)
		return
	}
	if len(wav) == 0 {
		h.fail(w, r, transcriptionerrors.ErrInvalidAudio)
		return
	}
	// The slot is taken once the body is in: a client still uploading must not make the others wait.
	select {
	case h.running <- struct{}{}:
		defer func() { <-h.running }()
	default:
		w.Header().Set("Retry-After", retryAfterSeconds)
		h.write(w, http.StatusTooManyRequests, dto.ErrorResponse{Error: msgBusy.T(r.Context())})
		return
	}
	transcript, err := h.transcribe.Execute(r.Context(), application.TranscribeInput{WAV: wav})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.write(w, http.StatusOK, dto.TranscriptionResponse{Text: transcript.Text})
}

// fail answers the status and translated text of a domain error. Anything else is a 500 whose detail stays in the log.
func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, message := http.StatusInternalServerError, msgFailed
	switch {
	case errors.Is(err, transcriptionerrors.ErrInvalidAudio):
		status, message = http.StatusBadRequest, msgInvalidAudio
	case errors.Is(err, transcriptionerrors.ErrAudioTooLong):
		status, message = http.StatusRequestEntityTooLarge, msgTooLong
	case errors.Is(err, transcriptionerrors.ErrNoSpeech):
		status, message = http.StatusUnprocessableEntity, msgNoSpeech
	case errors.Is(err, transcriptionerrors.ErrUnavailable):
		status, message = http.StatusServiceUnavailable, msgUnavailable
	case errors.Is(err, context.Canceled):
		// The client left: nobody reads the answer.
		status, message = http.StatusRequestTimeout, msgFailed
	}
	if status >= http.StatusInternalServerError {
		h.logger.Error("transcription failed", "status", status, "error", err)
	}
	h.write(w, status, dto.ErrorResponse{Error: message.T(r.Context())})
}

func (h *Handlers) write(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.logger.Warn("write transcription answer", "error", err)
	}
}
