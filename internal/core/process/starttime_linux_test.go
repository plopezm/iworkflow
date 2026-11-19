//go:build linux

package process

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// startStampLine builds a /proc stat line: after comm come the state and 18 more fields, then field 22.
func startStampLine(comm string) string {
	return "123 (" + comm + ") S " + strings.Repeat("0 ", 18) + "98765 0 0"
}

func TestParseStartStamp(t *testing.T) {
	tests := []struct {
		name string
		stat string
		want string
	}{
		{"plain command", startStampLine("sleep"), "98765"},
		{"command with spaces", startStampLine("my server"), "98765"},
		{"command with parentheses", startStampLine("a) (b"), "98765"},
		{"too few fields", "123 (x) S 1 2 3", ""},
		{"no command name", "123 S 1 2 3", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseStartStamp(tt.stat))
		})
	}
}

func TestProcessStartStampOfAGoneProcessIsEmpty(t *testing.T) {
	assert.Empty(t, processStartStamp(1<<30))
}
