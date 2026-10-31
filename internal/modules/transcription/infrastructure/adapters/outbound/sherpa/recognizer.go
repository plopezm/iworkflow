// Package sherpa is the outbound adapter that recognizes speech with sherpa-onnx and the pre-converted
// Qwen3-ASR model. The cgo binding is confined to engine_sherpa.go behind the `sherpa` build tag; without the
// tag the adapter compiles CGO-free and reports ErrUnavailable.
package sherpa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
)

// modelFiles are the files of the Qwen3-ASR model folder that must exist for the recognizer to load.
var modelFiles = [...]string{
	"conv_frontend.onnx",
	"encoder.int8.onnx",
	"decoder.int8.onnx",
	filepath.Join("tokenizer", "vocab.json"),
	filepath.Join("tokenizer", "merges.txt"),
}

// engine is one loaded recognizer. It is not safe for concurrent use: Recognizer serializes the calls.
type engine interface {
	decode(samples []float32, sampleRate int) (string, error)
	close()
}

// Recognizer loads the model on the first transcription (the model takes seconds and hundreds of MB, so a
// server that never transcribes never pays for it) and keeps it for the next ones. A failed load is not
// remembered: downloading the model later makes the next call work.
type Recognizer struct {
	modelDir string

	mu     sync.Mutex
	engine engine
}

// NewRecognizer builds the adapter for the model folder. It reads nothing until the first Recognize.
func NewRecognizer(modelDir string) *Recognizer {
	return &Recognizer{modelDir: modelDir}
}

// Recognize transcribes the samples. Calls run one at a time: the model uses all the threads it is given.
func (r *Recognizer) Recognize(ctx context.Context, samples []float32, sampleRate int) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("recognize: %w", err)
	}
	if r.engine == nil {
		loaded, err := r.load()
		if err != nil {
			return "", err
		}
		r.engine = loaded
	}
	text, err := r.engine.decode(samples, sampleRate)
	if err != nil {
		return "", fmt.Errorf("decode audio: %w", err)
	}
	return text, nil
}

// Close frees the loaded model, if any.
func (r *Recognizer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.engine != nil {
		r.engine.close()
		r.engine = nil
	}
}

func (r *Recognizer) load() (engine, error) {
	for _, name := range modelFiles {
		path := filepath.Join(r.modelDir, name)
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("model file %s (run `make model`): %w", path, transcriptionerrors.ErrUnavailable)
		}
	}
	loaded, err := newEngine(r.modelDir)
	if err != nil {
		return nil, err
	}
	return loaded, nil
}
