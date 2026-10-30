package application

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Rust: trigger_successors::delay_grows_exponentially_and_is_capped
func TestBackoffDelayGrowsExponentiallyAndIsCapped(t *testing.T) {
	t.Parallel()
	seconds := []int64{15, 30, 60, 120, 240, 480, 600, 600}
	for index, want := range seconds {
		attempts := uint32(index + 1)
		assert.Equal(t, time.Duration(want)*time.Second, backoffDelay(attempts), "after %d failures", attempts)
	}
	assert.Equal(t, 600*time.Second, backoffDelay(math.MaxUint32))
}
