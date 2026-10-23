package components_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/components"
)

func TestParsePositiveNumberAcceptsPositiveIntegersOnly(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  uint32
		ok    bool
	}{
		{name: "padded integer", input: " 2048 ", want: 2048, ok: true},
		{name: "zero", input: "0", ok: false},
		{name: "empty", input: "", ok: false},
		{name: "negative", input: "-5", ok: false},
		{name: "not a number", input: "abc", ok: false},
		{name: "overflows uint32", input: "4294967296", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := components.ParsePositiveNumber(tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
