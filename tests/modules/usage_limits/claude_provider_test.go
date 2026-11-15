package usagelimits_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/outbound/claude"
)

const secretToken = "sk-ant-oat-SECRET"

const usageBody = `{"five_hour":{"utilization":19.0,"resets_at":"2026-11-15T12:00:00.483767+00:00"},` +
	`"seven_day":{"utilization":26.5,"resets_at":"2026-11-20T12:00:00+00:00"},"seven_day_opus":null}`

func writeCredentials(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".credentials.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func validCredentials(t *testing.T) string {
	t.Helper()
	return writeCredentials(t, `{"claudeAiOauth":{"accessToken":"`+secretToken+`"}}`)
}

func newClaude(path string, server *httptest.Server) *claude.Provider {
	return claude.NewProvider(claude.Config{
		CredentialsPath: path,
		Endpoint:        server.URL,
		Client:          server.Client(),
		Clock:           stoppedClock{now: testNow},
	})
}

func TestClaudeFetchMapsTheTwoWindowsAndSendsTheToken(t *testing.T) {
	var gotAuth, gotBeta string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotBeta = r.Header.Get("Authorization"), r.Header.Get("anthropic-beta")
		_, _ = w.Write([]byte(usageBody))
	}))
	defer server.Close()

	usage, err := newClaude(validCredentials(t), server).Fetch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Bearer "+secretToken, gotAuth)
	assert.Equal(t, "oauth-2025-04-20", gotBeta)
	assert.Equal(t, models.ProviderClaude, usage.Provider())
	assert.Equal(t, testNow, usage.FetchedAt())
	windows := usage.Windows()
	require.Len(t, windows, 2)
	assert.Equal(t, models.WindowFiveHour, windows[0].Name())
	assert.InDelta(t, 19.0, windows[0].UsedPct(), 0.001)
	assert.Equal(t, time.Date(2026, 11, 15, 12, 0, 0, 483767000, time.UTC), windows[0].ResetsAt().UTC())
	assert.Equal(t, models.WindowWeek, windows[1].Name())
	assert.InDelta(t, 26.5, windows[1].UsedPct(), 0.001)
}

func TestClaudeFetchSkipsAWindowTheEndpointHasNoDataFor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"five_hour":null,"seven_day":{"utilization":3,"resets_at":null}}`))
	}))
	defer server.Close()

	usage, err := newClaude(validCredentials(t), server).Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, usage.Windows(), 1)
	assert.Equal(t, models.WindowWeek, usage.Windows()[0].Name())
	assert.True(t, usage.Windows()[0].ResetsAt().IsZero())
}

func TestClaudeFetchIsNotConfiguredWithoutCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the endpoint must not be called without a token")
	}))
	defer server.Close()

	tests := map[string]string{
		"missing file":  filepath.Join(t.TempDir(), "absent.json"),
		"empty token":   writeCredentials(t, `{"claudeAiOauth":{"accessToken":""}}`),
		"api key login": writeCredentials(t, `{}`),
	}
	for name, path := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newClaude(path, server).Fetch(context.Background())

			require.ErrorIs(t, err, usageerrors.ErrNotConfigured)
		})
	}
}

func TestClaudeFetchDegradesToUnavailableAndNeverLeaksTheToken(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"unauthorized": func(w http.ResponseWriter, _ *http.Request) { http.Error(w, secretToken, http.StatusUnauthorized) },
		"server error": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) },
		"not json":     func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(secretToken)) },
		"no windows":   func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"five_hour":null}`)) },
		"bad reset": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"five_hour":{"utilization":1,"resets_at":"soon"}}`))
		},
	}
	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()

			_, err := newClaude(validCredentials(t), server).Fetch(context.Background())

			require.ErrorIs(t, err, usageerrors.ErrUnavailable)
			assert.NotContains(t, err.Error(), secretToken)
		})
	}
}

func TestClaudeFetchIsUnavailableWhenTheServerIsDown(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	provider := newClaude(validCredentials(t), server)
	server.Close()

	_, err := provider.Fetch(context.Background())

	require.ErrorIs(t, err, usageerrors.ErrUnavailable)
	assert.NotContains(t, err.Error(), secretToken)
}
