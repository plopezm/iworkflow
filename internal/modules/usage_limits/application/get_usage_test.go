package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/usage_limits/application"
	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
)

func usageOf(t *testing.T, id models.ProviderID, pct float64) models.ProviderUsage {
	t.Helper()
	window, err := models.NewWindow(models.WindowFiveHour, pct, time.Time{})
	require.NoError(t, err)
	usage, err := models.NewProviderUsage(id, []models.Window{window}, time.Unix(0, 0))
	require.NoError(t, err)
	return usage
}

func TestGetUsageAggregatesProviders(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name            string
		providers       []outbound.UsageProviderPort
		wantUsages      []models.ProviderID
		wantUnavailable []models.ProviderID
	}{
		{
			name: "keeps the provider order",
			providers: []outbound.UsageProviderPort{
				fakeProvider{id: models.ProviderClaude, usage: usageOf(t, models.ProviderClaude, 10)},
				fakeProvider{id: models.ProviderCodex, usage: usageOf(t, models.ProviderCodex, 20)},
			},
			wantUsages: []models.ProviderID{models.ProviderClaude, models.ProviderCodex},
		},
		{
			name: "hides a provider that is not configured",
			providers: []outbound.UsageProviderPort{
				fakeProvider{id: models.ProviderClaude, err: usageerrors.ErrNotConfigured},
				fakeProvider{id: models.ProviderCodex, usage: usageOf(t, models.ProviderCodex, 20)},
			},
			wantUsages: []models.ProviderID{models.ProviderCodex},
		},
		{
			name: "reports a failing provider without losing the others",
			providers: []outbound.UsageProviderPort{
				fakeProvider{id: models.ProviderClaude, err: boom},
				fakeProvider{id: models.ProviderCodex, usage: usageOf(t, models.ProviderCodex, 20)},
			},
			wantUsages:      []models.ProviderID{models.ProviderCodex},
			wantUnavailable: []models.ProviderID{models.ProviderClaude},
		},
		{name: "no providers gives an empty report"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report, err := application.NewGetUsage(tc.providers...).Execute(context.Background())

			require.NoError(t, err)
			var got []models.ProviderID
			for _, usage := range report.Usages {
				got = append(got, usage.Provider())
			}
			assert.Equal(t, tc.wantUsages, got)
			assert.Equal(t, tc.wantUnavailable, report.Unavailable)
		})
	}
}

func TestGetUsageStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := application.NewGetUsage(fakeProvider{id: models.ProviderClaude}).Execute(ctx)

	require.ErrorIs(t, err, context.Canceled)
}
