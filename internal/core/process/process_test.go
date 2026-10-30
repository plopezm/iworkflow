package process

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskkillArgsKillTheWholeTreeByPID(t *testing.T) {
	assert.Equal(t, []string{"/PID", "42", "/T", "/F"}, taskkillArgs(42))
}
