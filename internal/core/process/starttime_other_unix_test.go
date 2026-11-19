//go:build unix && !linux

package process

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The pid-reuse guard needs a stable stamp for a live process and an empty one for a gone pid.
func TestProcessStartStampIsStableForALiveProcessAndEmptyForAGoneOne(t *testing.T) {
	first := processStartStamp(os.Getpid())
	assert.NotEmpty(t, first)
	assert.Equal(t, first, processStartStamp(os.Getpid()))
	assert.Empty(t, processStartStamp(1<<30))
}
