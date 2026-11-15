// Package codex is the facade over the ChatGPT (Codex) subscription usage: Codex writes the rate limits of the
// account into the rollout files of its sessions, so the facade reads the newest one and needs no network.
package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
)

const (
	// tailBytes is how much of the end of a rollout is searched: the latest rate limits are near the end.
	tailBytes     int64 = 4 << 20
	rateLimitsKey       = `"rate_limits":{`
	// fiveHourMaxMinutes separates the short window from the weekly one when the file says its length.
	fiveHourMaxMinutes float64 = 720
	// maxRolloutsTried bounds how many of the newest rollouts are searched for rate limits.
	maxRolloutsTried = 10
)

// Config is what the facade needs.
type Config struct {
	// SessionsDir is ~/.codex/sessions.
	SessionsDir string
	Clock       outbound.ClockPort
}

// Provider implements UsageProviderPort for Codex.
type Provider struct {
	sessionsDir string
	clock       outbound.ClockPort
}

var _ outbound.UsageProviderPort = (*Provider)(nil)

// NewProvider builds the facade.
func NewProvider(cfg Config) *Provider {
	return &Provider{sessionsDir: cfg.SessionsDir, clock: cfg.Clock}
}

// ID is the provider this facade reads.
func (*Provider) ID() models.ProviderID { return models.ProviderCodex }

type limitDTO struct {
	UsedPercent      *float64 `json:"used_percent"`
	WindowMinutes    *float64 `json:"window_minutes"`
	ResetsAt         *int64   `json:"resets_at"`
	ResetsInSeconds  *int64   `json:"resets_in_seconds"`
	ResetAfterSecond *int64   `json:"resets_after_seconds"`
}

type eventDTO struct {
	Timestamp string `json:"timestamp"`
	Payload   struct {
		RateLimits struct {
			Primary   *limitDTO `json:"primary"`
			Secondary *limitDTO `json:"secondary"`
		} `json:"rate_limits"`
	} `json:"payload"`
}

// Fetch returns the rate limits of the newest rollout. No sessions directory, no rollout or no rate limits in it
// (an API-key account has none) means the provider is not configured.
func (p *Provider) Fetch(ctx context.Context) (models.ProviderUsage, error) {
	files, err := p.rolloutsNewestFirst(ctx)
	if err != nil {
		return models.ProviderUsage{}, err
	}
	line, err := firstRateLimitsLine(files)
	if err != nil {
		return models.ProviderUsage{}, err
	}
	var event eventDTO
	if err := json.Unmarshal(line, &event); err != nil {
		return models.ProviderUsage{}, fmt.Errorf("decode the rate limits: %w", usageerrors.ErrUnavailable)
	}
	stamp, _ := time.Parse(time.RFC3339Nano, event.Timestamp) // an unreadable stamp only loses relative resets
	now := p.clock.Now()
	windows := make([]models.Window, 0, 2)
	for position, limit := range []*limitDTO{event.Payload.RateLimits.Primary, event.Payload.RateLimits.Secondary} {
		window, ok, err := toWindow(position, limit, stamp)
		if err != nil {
			return models.ProviderUsage{}, fmt.Errorf("%w: %w", usageerrors.ErrUnavailable, err)
		}
		// A window whose reset has passed was renewed since the rollout was written: its number is old news.
		if ok && (window.ResetsAt().IsZero() || window.ResetsAt().After(now)) {
			windows = append(windows, window)
		}
	}
	if len(windows) == 0 {
		return models.ProviderUsage{}, fmt.Errorf("the last Codex rate limits are already renewed: %w", usageerrors.ErrNotConfigured)
	}
	usage, err := models.NewProviderUsage(models.ProviderCodex, windows, now)
	if err != nil {
		return models.ProviderUsage{}, fmt.Errorf("%w: %w", usageerrors.ErrUnavailable, err)
	}
	return usage, nil
}

// rolloutsNewestFirst lists the newest rollout files, most recently modified first. The sessions are kept in
// year/month/day folders, so only the newest folders are read, never the whole history; a folder that cannot be
// read is skipped. No rollout means the provider is not configured.
func (p *Provider) rolloutsNewestFirst(ctx context.Context) ([]string, error) {
	if _, err := os.Stat(p.sessionsDir); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no Codex sessions: %w", usageerrors.ErrNotConfigured)
	}
	found, err := collectNewest(ctx, p.sessionsDir, maxRolloutsTried)
	if err != nil {
		return nil, fmt.Errorf("list the Codex sessions: %w", usageerrors.ErrUnavailable)
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no Codex rollout: %w", usageerrors.ErrNotConfigured)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].modTime.After(found[j].modTime) })
	paths := make([]string, 0, min(len(found), maxRolloutsTried))
	for _, entry := range found[:min(len(found), maxRolloutsTried)] {
		paths = append(paths, entry.path)
	}
	return paths, nil
}

type rollout struct {
	path    string
	modTime time.Time
}

// collectNewest gathers the rollouts of dir, visiting the sub-folders in descending name order (the newest dates
// first) and stopping once enough files are known; the folder being read is always finished.
func collectNewest(ctx context.Context, dir string, want int) ([]rollout, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("walk the sessions: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil //nolint:nilerr // an unreadable folder is skipped, the others still count
	}
	var found []rollout
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			found = append(found, rollout{path: filepath.Join(dir, name), modTime: info.ModTime()})
		}
	}
	for index := len(entries) - 1; index >= 0 && len(found) < want; index-- {
		if !entries[index].IsDir() {
			continue
		}
		deeper, err := collectNewest(ctx, filepath.Join(dir, entries[index].Name()), want-len(found))
		if err != nil {
			return nil, err
		}
		found = append(found, deeper...)
	}
	return found, nil
}

// firstRateLimitsLine returns the rate limits line of the newest rollout that has one: a fresh session may not have
// any yet while the one before it does.
func firstRateLimitsLine(files []string) ([]byte, error) {
	for _, file := range files {
		line, err := lastRateLimitsLine(file)
		if errors.Is(err, usageerrors.ErrNotConfigured) {
			continue
		}
		return line, err
	}
	return nil, fmt.Errorf("no rollout has rate limits: %w", usageerrors.ErrNotConfigured)
}

// lastRateLimitsLine returns the last line of the file tail that carries rate limits.
func lastRateLimitsLine(path string) ([]byte, error) {
	handle, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open the rollout: %w", usageerrors.ErrUnavailable)
	}
	defer func() { _ = handle.Close() }() // read only: a close failure changes nothing
	info, err := handle.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat the rollout: %w", usageerrors.ErrUnavailable)
	}
	offset := max(info.Size()-tailBytes, 0)
	tail := make([]byte, info.Size()-offset)
	if _, err := handle.ReadAt(tail, offset); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read the rollout: %w", usageerrors.ErrUnavailable)
	}
	index := bytes.LastIndex(tail, []byte(rateLimitsKey))
	if index < 0 {
		return nil, fmt.Errorf("the rollout has no rate limits: %w", usageerrors.ErrNotConfigured)
	}
	start := bytes.LastIndexByte(tail[:index], '\n') + 1
	if start == 0 && offset > 0 {
		return nil, fmt.Errorf("the rate limits line is cut: %w", usageerrors.ErrUnavailable)
	}
	end := bytes.IndexByte(tail[index:], '\n')
	if end < 0 {
		return tail[start:], nil
	}
	return tail[start : index+end], nil
}

// toWindow maps one limit. The length of the window decides its name when the file gives it; otherwise the
// primary limit is the short window and the secondary one is the weekly window.
func toWindow(position int, limit *limitDTO, stamp time.Time) (models.Window, bool, error) {
	if limit == nil || limit.UsedPercent == nil {
		return models.Window{}, false, nil
	}
	name := models.WindowFiveHour
	switch {
	case limit.WindowMinutes != nil && *limit.WindowMinutes > fiveHourMaxMinutes:
		name = models.WindowWeek
	case limit.WindowMinutes == nil && position == 1:
		name = models.WindowWeek
	}
	var resetsAt time.Time
	switch {
	case limit.ResetsAt != nil:
		resetsAt = time.Unix(*limit.ResetsAt, 0).UTC()
	case !stamp.IsZero() && limit.ResetsInSeconds != nil:
		resetsAt = stamp.Add(time.Duration(*limit.ResetsInSeconds) * time.Second)
	case !stamp.IsZero() && limit.ResetAfterSecond != nil:
		resetsAt = stamp.Add(time.Duration(*limit.ResetAfterSecond) * time.Second)
	}
	window, err := models.NewWindow(name, *limit.UsedPercent, resetsAt)
	if err != nil {
		return models.Window{}, false, fmt.Errorf("map window %s: %w", name, err)
	}
	return window, true, nil
}
