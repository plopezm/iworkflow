//go:build !sherpa

package sherpa

import (
	"fmt"

	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
)

// newEngine reports that this binary was built without the sherpa-onnx binding.
func newEngine(string) (engine, error) {
	return nil, fmt.Errorf("built without the `sherpa` tag: %w", transcriptionerrors.ErrUnavailable)
}
