//go:build sherpa

package transcription_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/transcription/application"
	"iworkflow/internal/modules/transcription/infrastructure/adapters/outbound/sherpa"
)

// modelDir is the Qwen3-ASR model the tests transcribe with; the test skips when `make model` has not run.
func modelDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("IWF_ASR_MODEL_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		dir = filepath.Join(home, ".iworkflow", "models", "sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25")
	}
	if _, err := os.Stat(filepath.Join(dir, "decoder.int8.onnx")); err != nil {
		t.Skipf("model not downloaded in %s (run `make model`)", dir)
	}
	return dir
}

func TestQwen3ASRTranscribesASpanishRecording(t *testing.T) {
	dir := modelDir(t)
	wav, err := os.ReadFile(filepath.Join(dir, "test_wavs", "es1.wav"))
	require.NoError(t, err)
	recognizer := sherpa.NewRecognizer(dir)
	defer recognizer.Close()

	transcript, err := application.NewTranscribeAudio(recognizer).Execute(context.Background(), application.TranscribeInput{WAV: wav})

	require.NoError(t, err)
	// The model transcript.txt reads: "Esta prenda es amplia, recomiendo elegir una talla menor a la habitual."
	require.Contains(t, strings.ToLower(transcript.Text), "prenda")
	require.Contains(t, strings.ToLower(transcript.Text), "talla")
}
