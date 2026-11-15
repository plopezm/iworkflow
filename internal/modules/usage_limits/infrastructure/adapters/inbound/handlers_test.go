package inbound_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/usage_limits/application"
	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/inbound"
)

var fixedNow = time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// fakeProvider answers a fixed usage or error.
type fakeProvider struct {
	id    models.ProviderID
	usage models.ProviderUsage
	err   error
}

func (f fakeProvider) ID() models.ProviderID { return f.id }

func (f fakeProvider) Fetch(context.Context) (models.ProviderUsage, error) { return f.usage, f.err }

func claudeUsage(t *testing.T, stale bool) models.ProviderUsage {
	t.Helper()
	win, err := models.NewWindow(models.WindowFiveHour, 13, fixedNow.Add(2*time.Hour+10*time.Minute))
	require.NoError(t, err)
	usage, err := models.NewProviderUsage(models.ProviderClaude, []models.Window{win}, fixedNow)
	require.NoError(t, err)
	if stale {
		return usage.AsStale()
	}
	return usage
}

// serve mounts the handlers over the providers and asks for the bar with ctx.
func serve(t *testing.T, ctx context.Context, providers ...outbound.UsageProviderPort) *httptest.ResponseRecorder {
	t.Helper()
	handlers := inbound.NewHandlers(application.NewGetUsage(providers...), fixedClock{now: fixedNow})
	router := chi.NewRouter()
	handlers.Mount(router)
	request := httptest.NewRequest(http.MethodGet, "/usage-limits/bar", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)
	return rec
}

func TestBarRendersTheProvidersAsCards(t *testing.T) {
	rec := serve(t, context.Background(), fakeProvider{id: models.ProviderClaude, usage: claudeUsage(t, false)})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	body := rec.Body.String()
	assert.Contains(t, body, `data-usage-provider="claude"`)
	assert.Contains(t, body, `data-usage-window="5h"`)
	assert.Contains(t, body, `data-level="normal"`)
	assert.Contains(t, body, "13%")
	assert.NotContains(t, body, "data-usage-stale")
}

func TestBarIsEmptyAndHiddenWhenNoProviderIsConfigured(t *testing.T) {
	rec := serve(t, context.Background(), fakeProvider{id: models.ProviderCodex, err: usageerrors.ErrNotConfigured})

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `id="usage-bar"`)
	assert.Contains(t, body, `hx-trigger="every 60s"`)
	assert.Contains(t, body, `class="hidden"`)
	assert.NotContains(t, body, "data-usage-provider")
}

func TestBarShowsAnUnavailableProviderAsAMutedLine(t *testing.T) {
	rec := serve(t, context.Background(), fakeProvider{id: models.ProviderCodex, err: usageerrors.ErrUnavailable})

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "data-usage-unavailable")
	assert.Contains(t, body, "Codex: unavailable")
	assert.NotContains(t, body, `class="hidden"`)
}

func TestBarMarksAStaleProvider(t *testing.T) {
	rec := serve(t, context.Background(), fakeProvider{id: models.ProviderClaude, usage: claudeUsage(t, true)})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "data-usage-stale")
}

func TestBarWritesNothingWhenTheRequestIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rec := serve(t, ctx, fakeProvider{id: models.ProviderClaude, usage: claudeUsage(t, false)})

	assert.Empty(t, rec.Body.String())
}
