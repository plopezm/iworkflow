//go:build linux

package process

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// statLine builds a /proc/<pid>/stat line whose starttime (field 22) is start. The fields after the closing
// parenthesis start at field 3 (the state), so starttime is the 20th of them.
func statLine(comm string, start string) string {
	fields := make([]string, 30)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0] = "S"
	fields[19] = start
	return "1234 (" + comm + ") " + strings.Join(fields, " ")
}

func TestStartTimeReadsField22AfterTheLastClosingParenthesis(t *testing.T) {
	cases := map[string]struct {
		stat string
		want string
	}{
		"plain name":            {statLine("sleep", "98765"), "98765"},
		"name with spaces":      {statLine("Web Content", "42"), "42"},
		"name with parentheses": {statLine("my (weird) name", "7"), "7"},
		"short line":            {"1234 (sleep) S 1 1234", ""},
		"no closing paren":      {"1234 (sleep S 1", ""},
		"empty":                 {"", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, startTime(tc.stat))
		})
	}
}
