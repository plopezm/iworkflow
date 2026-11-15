// Package state builds the view model of the usage bar from the data the handlers read. It holds no
// business logic: the windows, levels and staleness come from the use case.
package state

import (
	"math"
	"time"

	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/inbound/dto"
)

// BarState is the immutable view model of the usage bar.
type BarState struct {
	// Hidden is true when there is nothing to show; the bar still polls so it can appear later.
	Hidden      bool
	Providers   []ProviderState
	Unavailable []string
}

// ProviderState is one provider card.
type ProviderState struct {
	ID      string
	Name    string
	Stale   bool
	Windows []WindowState
}

// WindowState is one window row of a provider card.
type WindowState struct {
	Name  string
	Pct   int
	Level string
	// ResetIn is the time left until the window renews; only meaningful when HasReset is true.
	ResetIn  time.Duration
	HasReset bool
}

// Bar builds the view model from the report, measuring the time left until each reset from now.
func Bar(report dto.ReportDTO, now time.Time) BarState {
	bar := BarState{}
	for _, provider := range report.Providers {
		card := ProviderState{ID: provider.ID, Name: providerName(provider.ID), Stale: provider.Stale}
		for _, window := range provider.Windows {
			card.Windows = append(card.Windows, windowState(window, now))
		}
		bar.Providers = append(bar.Providers, card)
	}
	for _, id := range report.Unavailable {
		bar.Unavailable = append(bar.Unavailable, providerName(id))
	}
	bar.Hidden = len(bar.Providers) == 0 && len(bar.Unavailable) == 0
	return bar
}

func windowState(window dto.WindowDTO, now time.Time) WindowState {
	out := WindowState{Name: window.Name, Pct: int(math.Round(window.UsedPct)), Level: window.Level}
	if !window.ResetsAt.IsZero() {
		out.HasReset = true
		out.ResetIn = max(window.ResetsAt.Sub(now), 0)
	}
	return out
}

// providerName is the brand shown for a provider identifier; unknown identifiers are shown as they are.
func providerName(id string) string {
	switch id {
	case "claude":
		return "Claude"
	case "codex":
		return "Codex"
	default:
		return id
	}
}
