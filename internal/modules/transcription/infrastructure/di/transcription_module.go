// Package di is the composition root of the transcription module.
package di

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/modules/transcription/application"
	"iworkflow/internal/modules/transcription/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/transcription/infrastructure/adapters/outbound/sherpa"
)

// Deps are the resources the module needs from the composition root.
type Deps struct {
	// ModelDir is the folder of the Qwen3-ASR model (`make model` downloads it).
	ModelDir string
	Logger   *slog.Logger
}

// Module exposes the use case of the transcription module and its routes.
type Module struct {
	Transcribe *application.TranscribeAudio

	recognizer *sherpa.Recognizer
	handlers   *inbound.Handlers
}

// NewModule wires the sherpa-onnx recognizer into the use case and the use case into the handlers. The model is
// not read until the first transcription.
func NewModule(deps Deps) *Module {
	recognizer := sherpa.NewRecognizer(deps.ModelDir)
	transcribe := application.NewTranscribeAudio(recognizer)
	return &Module{
		Transcribe: transcribe,
		recognizer: recognizer,
		handlers:   inbound.NewHandlers(transcribe, deps.Logger),
	}
}

// Routes mounts the endpoint. It satisfies web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Routes(r)
}

// Shutdown frees the loaded model.
func (m *Module) Shutdown(context.Context) error {
	m.recognizer.Close()
	return nil
}
