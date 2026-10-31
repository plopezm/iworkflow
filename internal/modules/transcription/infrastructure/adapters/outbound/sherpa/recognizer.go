// Package sherpa is the outbound adapter that recognizes speech with sherpa-onnx and the pre-converted
// Qwen3-ASR model. The cgo binding is confined to engine_sherpa.go behind the `sherpa` build tag; without the
// tag the adapter compiles CGO-free and reports ErrUnavailable.
package sherpa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
// remembered: downloading the model later makes the next call work. After Close it stays unavailable.
type Recognizer struct {
	modelDir string

	// slot is the lock around engine and closed: a one-place channel, so a caller waiting for it can still give up when
	// its context ends (a sync.Mutex cannot be abandoned).
	slot   chan struct{}
	engine engine
	closed bool
}

// NewRecognizer builds the adapter for the model folder. It reads nothing until the first Recognize.
func NewRecognizer(modelDir string) *Recognizer {
	return &Recognizer{modelDir: modelDir, slot: make(chan struct{}, 1)}
}

// acquire takes the slot, or returns the error of ctx when it ends first.
func (r *Recognizer) acquire(ctx context.Context) error {
	select {
	case r.slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for the recognizer: %w", ctx.Err())
	}
}

func (r *Recognizer) release() { <-r.slot }

// Recognize transcribes the samples. Calls run one at a time: the model uses all the threads it is given.
func (r *Recognizer) Recognize(ctx context.Context, samples []float32, sampleRate int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("recognize: %w", err)
	}
	if err := r.acquire(ctx); err != nil {
		return "", err
	}
	defer r.release()
	if r.closed {
		return "", fmt.Errorf("the recognizer was closed: %w", transcriptionerrors.ErrUnavailable)
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

// Close frees the loaded model, if any, and makes every later Recognize unavailable. It waits for a running
// transcription to finish.
func (r *Recognizer) Close() {
	r.slot <- struct{}{}
	defer r.release()
	r.closed = true
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
