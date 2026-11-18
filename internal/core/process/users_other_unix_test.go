//go:build unix && !linux

package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUsersOfReportsATimedOutLsofEvenWithPartialOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := usersOf(ctx, []string{t.TempDir()})

	assert.Error(t, err)
}
