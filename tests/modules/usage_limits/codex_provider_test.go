package usagelimits_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/outbound/codex"
)

func tokenCount(stamp string, limits string) string {
	return `{"timestamp":"` + stamp + `","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":` + limits + `}}`
}

func writeRollout(t *testing.T, dir, name string, modTime time.Time, lines ...string) {
	t.Helper()
	path := filepath.Join(dir, "2026", "11", "15", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
}

func newCodex(dir string) *codex.Provider {
	return codex.NewProvider(codex.Config{SessionsDir: dir, Clock: stoppedClock{now: testNow}})
}

func TestCodexFetchReadsTheLastRateLimitsOfTheNewestRollout(t *testing.T) {
	dir := t.TempDir()
	writeRollout(t, dir, "rollout-old.jsonl", testNow.Add(-time.Hour),
		tokenCount("2026-11-15T01:00:00Z", `{"primary":{"used_percent":99,"window_minutes":300},"secondary":null}`))
	writeRollout(t, dir, "rollout-new.jsonl", testNow,
		tokenCount("2026-11-15T09:00:00Z", `{"primary":{"used_percent":1,"window_minutes":300},"secondary":{"used_percent":2,"window_minutes":10080}}`),
		`{"type":"event_msg","payload":{"type":"agent_message"}}`,
		tokenCount("2026-11-15T09:30:00Z",
			`{"primary":{"used_percent":13.0,"window_minutes":300,"resets_in_seconds":3600},`+
				`"secondary":{"used_percent":25.0,"window_minutes":10080,"resets_at":1763640000}}`),
		`{"type":"event_msg","payload":{"type":"user_message"}}`)

	usage, err := newCodex(dir).Fetch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, models.ProviderCodex, usage.Provider())
	windows := usage.Windows()
	require.Len(t, windows, 2)
	assert.Equal(t, models.WindowFiveHour, windows[0].Name())
	assert.InDelta(t, 13.0, windows[0].UsedPct(), 0.001)
	assert.Equal(t, time.Date(2026, 11, 15, 10, 30, 0, 0, time.UTC), windows[0].ResetsAt().UTC())
	assert.Equal(t, models.WindowWeek, windows[1].Name())
	assert.InDelta(t, 25.0, windows[1].UsedPct(), 0.001)
	assert.Equal(t, time.Unix(1763640000, 0).UTC(), windows[1].ResetsAt().UTC())
}

func TestCodexFetchNamesTheWindowsByPositionWhenTheFileGivesNoLength(t *testing.T) {
	dir := t.TempDir()
	writeRollout(t, dir, "rollout-a.jsonl", testNow,
		tokenCount("2026-11-15T09:00:00Z", `{"primary":{"used_percent":5},"secondary":{"used_percent":6}}`))

	usage, err := newCodex(dir).Fetch(context.Background())

	require.NoError(t, err)
	windows := usage.Windows()
	require.Len(t, windows, 2)
	assert.Equal(t, models.WindowFiveHour, windows[0].Name())
	assert.Equal(t, models.WindowWeek, windows[1].Name())
}

func TestCodexFetchIsNotConfiguredWithoutRateLimits(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	empty := t.TempDir()
	noLimits := t.TempDir()
	writeRollout(t, noLimits, "rollout-a.jsonl", testNow,
		`{"type":"event_msg","payload":{"type":"token_count","rate_limits":null}}`)

	for name, dir := range map[string]string{"missing dir": missing, "no rollout": empty, "null rate limits": noLimits} {
		t.Run(name, func(t *testing.T) {
			_, err := newCodex(dir).Fetch(context.Background())

			require.ErrorIs(t, err, usageerrors.ErrNotConfigured)
		})
	}
}

func TestCodexFetchIsUnavailableOnACorruptLine(t *testing.T) {
	dir := t.TempDir()
	writeRollout(t, dir, "rollout-a.jsonl", testNow, `{"payload":{"rate_limits":{"primary": BROKEN`)

	_, err := newCodex(dir).Fetch(context.Background())

	require.ErrorIs(t, err, usageerrors.ErrUnavailable)
}

func TestCodexFetchStopsOnACancelledContext(t *testing.T) {
	dir := t.TempDir()
	writeRollout(t, dir, "rollout-a.jsonl", testNow, tokenCount("2026-11-15T09:00:00Z", `{"primary":{"used_percent":5}}`))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newCodex(dir).Fetch(ctx)

	require.ErrorIs(t, err, usageerrors.ErrUnavailable)
}

func TestCodexFetchFallsBackToAnOlderRolloutWhenTheNewestHasNoRateLimitsYet(t *testing.T) {
	dir := t.TempDir()
	writeRollout(t, dir, "rollout-older.jsonl", testNow.Add(-time.Hour),
		tokenCount("2026-11-15T08:00:00Z", `{"primary":{"used_percent":33,"window_minutes":300},"secondary":null}`))
	writeRollout(t, dir, "rollout-fresh.jsonl", testNow, `{"type":"session_meta","payload":{}}`)

	usage, err := newCodex(dir).Fetch(context.Background())

	require.NoError(t, err)
	assert.InDelta(t, 33, usage.Windows()[0].UsedPct(), 0.001)
}

func TestCodexFetchSearchesOnlyTheTailOfAHugeRollout(t *testing.T) {
	dir := t.TempDir()
	filler := `{"type":"event_msg","payload":{"type":"agent_message","text":"` + strings.Repeat("x", 1<<20) + `"}}`
	writeRollout(t, dir, "rollout-huge.jsonl", testNow,
		tokenCount("2026-11-15T01:00:00Z", `{"primary":{"used_percent":1,"window_minutes":300}}`),
		filler, filler, filler, filler, filler,
		tokenCount("2026-11-15T09:00:00Z", `{"primary":{"used_percent":88,"window_minutes":300}}`))

	usage, err := newCodex(dir).Fetch(context.Background())

	require.NoError(t, err)
	assert.InDelta(t, 88, usage.Windows()[0].UsedPct(), 0.001)
}
