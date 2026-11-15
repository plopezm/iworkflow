package dto_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/inbound/dto"
)

func TestFromReportMapsProvidersWindowsAndUnavailable(t *testing.T) {
	resets := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	win5, err := models.NewWindow(models.WindowFiveHour, 13.4, resets)
	require.NoError(t, err)
	winWeek, err := models.NewWindow(models.WindowWeek, 95, time.Time{})
	require.NoError(t, err)
	usage, err := models.NewProviderUsage(models.ProviderClaude, []models.Window{win5, winWeek}, resets)
	require.NoError(t, err)
	report := models.Report{
		Usages:      []models.ProviderUsage{usage.AsStale()},
		Unavailable: []models.ProviderID{models.ProviderCodex},
	}

	got := dto.FromReport(report)

	require.Len(t, got.Providers, 1)
	provider := got.Providers[0]
	assert.Equal(t, "claude", provider.ID)
	assert.True(t, provider.Stale)
	require.Len(t, provider.Windows, 2)
	assert.Equal(t, dto.WindowDTO{Name: "5h", UsedPct: 13.4, Level: "normal", ResetsAt: resets}, provider.Windows[0])
	assert.Equal(t, dto.WindowDTO{Name: "week", UsedPct: 95, Level: "critical"}, provider.Windows[1])
	assert.Equal(t, []string{"codex"}, got.Unavailable)
}

func TestFromReportOfNothingIsEmpty(t *testing.T) {
	got := dto.FromReport(models.Report{})

	assert.Empty(t, got.Providers)
	assert.Empty(t, got.Unavailable)
}
