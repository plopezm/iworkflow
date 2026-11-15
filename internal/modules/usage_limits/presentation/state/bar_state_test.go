package state_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/usage_limits/presentation/state"
)

var now = time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

func TestBarRoundsThePercentageAndKeepsTheLevel(t *testing.T) {
	tests := []struct {
		name    string
		usedPct float64
		level   string
		want    int
	}{
		{name: "rounds down", usedPct: 13.4, level: "normal", want: 13},
		{name: "rounds up at half", usedPct: 69.5, level: "warn", want: 70},
		{name: "rounds up", usedPct: 90.6, level: "critical", want: 91},
		{name: "zero", usedPct: 0, level: "normal", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := state.Bar(report(tt.usedPct, tt.level, time.Time{}), now)

			require.Len(t, bar.Providers, 1)
			require.Len(t, bar.Providers[0].Windows, 1)
			window := bar.Providers[0].Windows[0]
			assert.Equal(t, tt.want, window.Pct)
			assert.Equal(t, tt.level, window.Level)
		})
	}
}

func TestBarComputesTheTimeLeftUntilTheReset(t *testing.T) {
	resets := now.Add(2*time.Hour + 10*time.Minute)

	window := state.Bar(report(10, "normal", resets), now).Providers[0].Windows[0]

	assert.True(t, window.HasReset)
	assert.Equal(t, 2*time.Hour+10*time.Minute, window.ResetIn)
}

func TestBarHasNoResetWhenTheTimeIsUnknown(t *testing.T) {
	window := state.Bar(report(10, "normal", time.Time{}), now).Providers[0].Windows[0]

	assert.False(t, window.HasReset)
	assert.Zero(t, window.ResetIn)
}

func TestBarClampsAResetThatAlreadyPassed(t *testing.T) {
	window := state.Bar(report(10, "normal", now.Add(-time.Minute)), now).Providers[0].Windows[0]

	assert.True(t, window.HasReset)
	assert.Zero(t, window.ResetIn)
}

func TestBarNamesTheProvidersAndMarksStaleOnes(t *testing.T) {
	input := dto.ReportDTO{
		Providers: []dto.ProviderDTO{
			{ID: "claude", Stale: true, Windows: []dto.WindowDTO{{Name: "5h", UsedPct: 1, Level: "normal"}}},
			{ID: "codex", Windows: []dto.WindowDTO{{Name: "week", UsedPct: 2, Level: "normal"}}},
		},
	}

	bar := state.Bar(input, now)

	require.Len(t, bar.Providers, 2)
	assert.Equal(t, "claude", bar.Providers[0].ID)
	assert.Equal(t, "Claude", bar.Providers[0].Name)
	assert.True(t, bar.Providers[0].Stale)
	assert.Equal(t, "Codex", bar.Providers[1].Name)
	assert.False(t, bar.Providers[1].Stale)
	assert.Equal(t, "week", bar.Providers[1].Windows[0].Name)
}

func TestBarIsHiddenOnlyWhenNothingIsShown(t *testing.T) {
	assert.True(t, state.Bar(dto.ReportDTO{}, now).Hidden)

	onlyUnavailable := state.Bar(dto.ReportDTO{Unavailable: []string{"codex"}}, now)

	assert.False(t, onlyUnavailable.Hidden)
	assert.Equal(t, []string{"Codex"}, onlyUnavailable.Unavailable)
}

func report(usedPct float64, level string, resets time.Time) dto.ReportDTO {
	return dto.ReportDTO{Providers: []dto.ProviderDTO{{
		ID:      "claude",
		Windows: []dto.WindowDTO{{Name: "5h", UsedPct: usedPct, Level: level, ResetsAt: resets}},
	}}}
}
