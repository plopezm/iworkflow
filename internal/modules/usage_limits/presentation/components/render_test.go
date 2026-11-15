package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/usage_limits/presentation/components"
	"iworkflow/internal/modules/usage_limits/presentation/state"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &buf))
	return buf.String()
}

func TestPlaceholderLoadsTheBarOnPageLoad(t *testing.T) {
	body := render(t, components.UsageBarPlaceholder())

	assert.Contains(t, body, `id="usage-bar"`)
	assert.Contains(t, body, `hx-get="/usage-limits/bar"`)
	assert.Contains(t, body, `hx-trigger="load"`)
	assert.Contains(t, body, `hx-swap="outerHTML"`)
}

func TestBarRendersOneCardPerProviderWithTheWindowRows(t *testing.T) {
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	bar := state.Bar(dto.ReportDTO{Providers: []dto.ProviderDTO{{
		ID: "claude",
		Windows: []dto.WindowDTO{
			{Name: "5h", UsedPct: 75, Level: "warn", ResetsAt: now.Add(2*time.Hour + 10*time.Minute)},
			{Name: "week", UsedPct: 13.2, Level: "normal", ResetsAt: now.Add(72 * time.Hour)},
		},
	}}}, now)

	body := render(t, components.UsageBar(bar))

	assert.Contains(t, body, `data-usage-provider="claude"`)
	assert.Contains(t, body, "Claude")
	assert.Contains(t, body, `data-usage-window="5h"`)
	assert.Contains(t, body, `data-level="warn"`)
	assert.Contains(t, body, "bg-warn")
	assert.Contains(t, body, "75%")
	assert.Contains(t, body, "↻ 2h10")
	assert.Contains(t, body, `data-usage-window="week"`)
	assert.Contains(t, body, "week")
	assert.Contains(t, body, "13%")
	assert.Contains(t, body, "↻ 3d")
	assert.Contains(t, body, "width: 75%")
	assert.NotContains(t, body, `class="hidden"`)
	assert.NotContains(t, body, "data-usage-stale")
}

func TestBarColorsTheFillByLevel(t *testing.T) {
	tests := []struct {
		level string
		class string
	}{
		{level: "normal", class: "bg-ok"},
		{level: "warn", class: "bg-warn"},
		{level: "critical", class: "bg-error"},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			bar := state.Bar(dto.ReportDTO{Providers: []dto.ProviderDTO{{
				ID:      "codex",
				Windows: []dto.WindowDTO{{Name: "5h", UsedPct: 50, Level: tt.level}},
			}}}, time.Now())

			body := render(t, components.UsageBar(bar))

			assert.Contains(t, body, tt.class)
		})
	}
}

func TestBarMarksAStaleProviderAndOmitsAnUnknownReset(t *testing.T) {
	bar := state.Bar(dto.ReportDTO{Providers: []dto.ProviderDTO{{
		ID:      "claude",
		Stale:   true,
		Windows: []dto.WindowDTO{{Name: "5h", UsedPct: 1, Level: "normal"}},
	}}}, time.Now())

	body := render(t, components.UsageBar(bar))

	assert.Contains(t, body, "data-usage-stale")
	assert.Contains(t, body, "stale")
	assert.NotContains(t, body, "↻")
}

func TestBarShowsUnavailableProvidersInTheRequestLanguage(t *testing.T) {
	bar := state.Bar(dto.ReportDTO{Unavailable: []string{"codex"}}, time.Now())

	body := render(t, components.UsageBar(bar))

	assert.Contains(t, body, "data-usage-unavailable")
	assert.Contains(t, body, "Codex: unavailable")
}

func TestEmptyBarIsHiddenAndKeepsPolling(t *testing.T) {
	bar := state.Bar(dto.ReportDTO{}, time.Now())

	body := render(t, components.UsageBar(bar))

	assert.Contains(t, body, `id="usage-bar"`)
	assert.Contains(t, body, `class="hidden"`)
	assert.Contains(t, body, `hx-trigger="every 60s"`)
	assert.Contains(t, body, `hx-get="/usage-limits/bar"`)
	assert.False(t, strings.Contains(body, "data-usage-provider"))
}
