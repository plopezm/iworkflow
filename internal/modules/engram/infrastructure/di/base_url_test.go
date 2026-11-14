package di

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{name: "unset is not configured", raw: "", wantErr: ErrNotConfigured},
		{name: "spaces only is not configured", raw: "   ", wantErr: ErrNotConfigured},
		{name: "not a url", raw: "not a url", wantErr: ErrInvalidBaseURL},
		{name: "ftp scheme", raw: "ftp://x", wantErr: ErrInvalidBaseURL},
		{name: "scheme without host", raw: "http://", wantErr: ErrInvalidBaseURL},
		{name: "space inside the host", raw: "http://127.0.0.1 :7437", wantErr: ErrInvalidBaseURL},
		{name: "loopback with port", raw: "http://127.0.0.1:7437", want: "http://127.0.0.1:7437"},
		{name: "trailing slash is dropped", raw: "http://127.0.0.1:7437/", want: "http://127.0.0.1:7437"},
		{name: "surrounding spaces are ignored", raw: "  https://engram.local  ", want: "https://engram.local"},
		{name: "user and password are rejected", raw: "http://user:pass@127.0.0.1:7437", wantErr: ErrInvalidBaseURL},
		{name: "user without password is rejected", raw: "https://token@engram.local", wantErr: ErrInvalidBaseURL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveBaseURL(tc.raw)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// The error must never echo the value: it may carry credentials.
func TestResolveBaseURLErrorsNeverEchoTheValue(t *testing.T) {
	const secret = "s3cr3t-token"
	for _, raw := range []string{"ftp://user:" + secret + "@host", "http://user:" + secret + "@ho st"} {
		_, err := ResolveBaseURL(raw)

		require.ErrorIs(t, err, ErrInvalidBaseURL)
		assert.NotContains(t, err.Error(), secret)
	}
}
