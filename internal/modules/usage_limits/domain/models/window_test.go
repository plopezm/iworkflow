package models_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
)

func TestNewWindowClampsAndValidates(t *testing.T) {
	reset := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		window  models.WindowName
		pct     float64
		want    float64
		wantErr bool
	}{
		{name: "keeps a normal value", window: models.WindowWeek, pct: 25, want: 25},
		{name: "clamps above 100", window: models.WindowWeek, pct: 130, want: 100},
		{name: "clamps below 0", window: models.WindowWeek, pct: -5, want: 0},
		{name: "rejects NaN", window: models.WindowWeek, pct: math.NaN(), wantErr: true},
		{name: "rejects an empty name", window: "", pct: 10, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			window, err := models.NewWindow(tc.window, tc.pct, reset)
			if tc.wantErr {
				require.ErrorIs(t, err, usageerrors.ErrInvalidUsage)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tc.want, window.UsedPct(), 0.001)
			assert.Equal(t, reset, window.ResetsAt())
		})
	}
}

func TestWindowLevelThresholds(t *testing.T) {
	tests := []struct {
		pct  float64
		want models.Level
	}{
		{0, models.LevelNormal}, {69.9, models.LevelNormal}, {70, models.LevelWarn},
		{89.9, models.LevelWarn}, {90, models.LevelCritical}, {100, models.LevelCritical},
	}
	for _, tc := range tests {
		window, err := models.NewWindow(models.WindowFiveHour, tc.pct, time.Time{})
		require.NoError(t, err)
		assert.Equal(t, tc.want, window.Level(), "pct %v", tc.pct)
	}
}

func TestProviderUsageNeedsProviderAndWindows(t *testing.T) {
	_, err := models.NewProviderUsage("", nil, time.Time{})
	require.ErrorIs(t, err, usageerrors.ErrInvalidUsage)
	_, err = models.NewProviderUsage(models.ProviderClaude, nil, time.Time{})
	require.ErrorIs(t, err, usageerrors.ErrInvalidUsage)
}

func TestProviderUsageAsStaleDoesNotMutateTheOriginal(t *testing.T) {
	window, err := models.NewWindow(models.WindowWeek, 1, time.Time{})
	require.NoError(t, err)
	usage, err := models.NewProviderUsage(models.ProviderClaude, []models.Window{window}, time.Time{})
	require.NoError(t, err)

	stale := usage.AsStale()

	assert.True(t, stale.Stale())
	assert.False(t, usage.Stale())
}
