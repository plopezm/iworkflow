//go:build sherpa

package sherpa

import (
	"fmt"
	"path/filepath"
	"runtime"

	sherpaonnx "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"

	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
)

const (
	featureDim   = 80
	maxTotalLen  = 2048
	maxNewTokens = 512
	// maxThreads leaves cores for the server: Qwen3-ASR 0.6B int8 decodes in real time with a few.
	maxThreads = 4
)

type sherpaEngine struct {
	recognizer *sherpaonnx.OfflineRecognizer
}

// newEngine creates the Qwen3-ASR offline recognizer from the model folder.
func newEngine(modelDir string) (engine, error) {
	config := sherpaonnx.OfflineRecognizerConfig{}
	config.FeatConfig.SampleRate = 16000
	config.FeatConfig.FeatureDim = featureDim
	config.ModelConfig.Qwen3ASR = sherpaonnx.OfflineQwen3ASRModelConfig{
		ConvFrontend: filepath.Join(modelDir, "conv_frontend.onnx"),
		Encoder:      filepath.Join(modelDir, "encoder.int8.onnx"),
		Decoder:      filepath.Join(modelDir, "decoder.int8.onnx"),
		Tokenizer:    filepath.Join(modelDir, "tokenizer"),
		MaxTotalLen:  maxTotalLen,
		MaxNewTokens: maxNewTokens,
		Temperature:  1e-6,
		TopP:         0.8,
		Seed:         42,
	}
	config.ModelConfig.NumThreads = min(runtime.NumCPU(), maxThreads)
	config.ModelConfig.Provider = "cpu"
	config.DecodingMethod = "greedy_search"

	recognizer := sherpaonnx.NewOfflineRecognizer(&config)
	if recognizer == nil {
		return nil, fmt.Errorf("sherpa-onnx could not load the model in %s: %w", modelDir, transcriptionerrors.ErrUnavailable)
	}
	return &sherpaEngine{recognizer: recognizer}, nil
}

func (e *sherpaEngine) decode(samples []float32, sampleRate int) (string, error) {
	if len(samples) == 0 {
		return "", nil
	}
	stream := sherpaonnx.NewOfflineStream(e.recognizer)
	defer sherpaonnx.DeleteOfflineStream(stream)
	stream.AcceptWaveform(sampleRate, samples)
	e.recognizer.Decode(stream)
	return stream.GetResult().Text, nil
}

func (e *sherpaEngine) close() {
	sherpaonnx.DeleteOfflineRecognizer(e.recognizer)
}
