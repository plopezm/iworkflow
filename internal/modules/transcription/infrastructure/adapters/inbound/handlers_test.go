package inbound_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/transcription/application"
	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
	"iworkflow/internal/modules/transcription/domain/models"
	"iworkflow/internal/modules/transcription/infrastructure/adapters/inbound"
)

type fakeTranscriber struct {
	transcript models.Transcript
	err        error
	got        []byte
	calls      int
}

func (f *fakeTranscriber) Execute(_ context.Context, input application.TranscribeInput) (models.Transcript, error) {
	f.calls++
	f.got = input.WAV
	return f.transcript, f.err
}

func post(t *testing.T, transcriber *fakeTranscriber, body string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Use(web.Locale)
	inbound.NewHandlers(transcriber).Routes(router)
	request := httptest.NewRequest(http.MethodPost, inbound.TranscriptionsPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "audio/wav")
	for name, values := range header {
		request.Header[name] = values
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	require.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	var payload map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}

func TestPostReturnsTheTranscriptAsJSON(t *testing.T) {
	transcriber := &fakeTranscriber{transcript: models.Transcript{Text: "Hola <mundo>"}}

	recorder := post(t, transcriber, "RIFFdata", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, map[string]string{"text": "Hola <mundo>"}, decode(t, recorder))
	require.Equal(t, []byte("RIFFdata"), transcriber.got)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestPostMapsDomainErrorsToStatusAndTranslatedMessage(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		es     string
	}{
		{"invalid audio", transcriptionerrors.ErrInvalidAudio, http.StatusBadRequest, "La grabación no es válida."},
		{"too long", transcriptionerrors.ErrAudioTooLong, http.StatusRequestEntityTooLarge, "La grabación es demasiado larga (máximo 60 segundos)."},
		{"no speech", transcriptionerrors.ErrNoSpeech, http.StatusUnprocessableEntity, "No se ha reconocido ninguna voz en la grabación."},
		{"unavailable", transcriptionerrors.ErrUnavailable, http.StatusServiceUnavailable, "La transcripción no está disponible en este servidor."},
		{"unexpected", errors.New("boom secret detail"), http.StatusInternalServerError, "No se ha podido transcribir la grabación."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := errors.Join(errors.New("context"), tt.err)

			recorder := post(t, &fakeTranscriber{err: wrapped}, "x", http.Header{"Cookie": {"iw-lang=es"}})

			require.Equal(t, tt.status, recorder.Code)
			require.Equal(t, map[string]string{"error": tt.es}, decode(t, recorder))
		})
	}
}

func TestPostWithoutLanguageAnswersInEnglish(t *testing.T) {
	recorder := post(t, &fakeTranscriber{err: transcriptionerrors.ErrNoSpeech}, "x", nil)

	require.Equal(t, "No speech was recognized in the recording.", decode(t, recorder)["error"])
}

func TestPostRejectsABodyBeyondTheLimitWithoutTranscribing(t *testing.T) {
	transcriber := &fakeTranscriber{}
	body := strings.Repeat("a", inbound.MaxBodyBytes+1)

	recorder := post(t, transcriber, body, nil)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Zero(t, transcriber.calls)
	require.Contains(t, decode(t, recorder)["error"], "60")
}

func TestPostRejectsAnEmptyBody(t *testing.T) {
	transcriber := &fakeTranscriber{}

	recorder := post(t, transcriber, "", nil)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, transcriber.calls)
}

func TestOtherMethodsAreNotAllowed(t *testing.T) {
	router := chi.NewRouter()
	inbound.NewHandlers(&fakeTranscriber{}).Routes(router)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, inbound.TranscriptionsPath, bytes.NewReader(nil)))

	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

// blockingTranscriber holds Execute until released, so a second request finds the endpoint busy.
type blockingTranscriber struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingTranscriber) Execute(context.Context, application.TranscribeInput) (models.Transcript, error) {
	close(b.started)
	<-b.release
	return models.Transcript{Text: "ok"}, nil
}

func TestPostAnswersTooManyRequestsWhileAnotherTranscriptionRuns(t *testing.T) {
	blocking := &blockingTranscriber{started: make(chan struct{}), release: make(chan struct{})}
	router := chi.NewRouter()
	router.Use(web.Locale)
	inbound.NewHandlers(blocking).Routes(router)
	first := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, inbound.TranscriptionsPath, strings.NewReader("a")))
	}()
	<-blocking.started

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, inbound.TranscriptionsPath, strings.NewReader("b")))

	require.Equal(t, http.StatusTooManyRequests, second.Code)
	require.Equal(t, "5", second.Header().Get("Retry-After"))
	require.Equal(t, "Another recording is being transcribed. Try again in a moment.", decode(t, second)["error"])
	close(blocking.release)
	<-finished
	require.Equal(t, http.StatusOK, first.Code)
}

func TestPostWithASlowBodyDoesNotHoldTheTranscriptionSlot(t *testing.T) {
	router := chi.NewRouter()
	router.Use(web.Locale)
	inbound.NewHandlers(&fakeTranscriber{transcript: models.Transcript{Text: "ok"}}).Routes(router)
	pipeReader, pipeWriter := io.Pipe()
	reading := make(chan struct{})
	slow := httptest.NewRequest(http.MethodPost, inbound.TranscriptionsPath, &signalReader{Reader: pipeReader, reading: reading})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		router.ServeHTTP(httptest.NewRecorder(), slow)
	}()

	<-reading // the slow request is now inside the handler, waiting for its body
	other := httptest.NewRecorder()
	router.ServeHTTP(other, httptest.NewRequest(http.MethodPost, inbound.TranscriptionsPath, strings.NewReader("fast")))

	require.Equal(t, http.StatusOK, other.Code, "a client still uploading must not make the others wait")
	require.NoError(t, pipeWriter.Close())
	<-finished
}

func TestPostFreesTheSlotWhenItFinishes(t *testing.T) {
	router := chi.NewRouter()
	router.Use(web.Locale)
	inbound.NewHandlers(&fakeTranscriber{transcript: models.Transcript{Text: "ok"}}).Routes(router)

	for range 3 {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, inbound.TranscriptionsPath, strings.NewReader("x")))
		require.Equal(t, http.StatusOK, recorder.Code)
	}
}

// signalReader closes reading on its first Read, so a test knows the handler reached the body.
type signalReader struct {
	io.Reader
	reading chan struct{}
	once    sync.Once
}

func (s *signalReader) Read(p []byte) (int, error) {
	s.once.Do(func() { close(s.reading) })
	return s.Reader.Read(p) //nolint:wrapcheck // a transparent reader wrapper
}
