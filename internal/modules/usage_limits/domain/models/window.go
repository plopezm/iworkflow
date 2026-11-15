// Package models holds the entities and value objects of the usage_limits module.
package models

import (
	"fmt"
	"math"
	"time"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
)

// WindowName identifies a rate-limit window of a subscription.
type WindowName string

// Known windows.
const (
	// WindowFiveHour is the rolling five-hour window.
	WindowFiveHour WindowName = "5h"
	// WindowWeek is the rolling seven-day window.
	WindowWeek WindowName = "week"
)

// Level classifies how close a window is to its limit.
type Level string

// Levels, from calm to exhausted.
const (
	LevelNormal   Level = "normal"
	LevelWarn     Level = "warn"
	LevelCritical Level = "critical"
)

const (
	warnFromPct     float64 = 70
	criticalFromPct float64 = 90
	maxPct          float64 = 100
)

// Window is the consumption of one rate-limit window. It is immutable.
type Window struct {
	name     WindowName
	usedPct  float64
	resetsAt time.Time
}

// NewWindow validates the data and builds the window. The percentage is clamped to 0..100 (providers may report
// slightly above the limit); a NaN or an empty name is invalid. A zero resetsAt means the reset time is unknown.
func NewWindow(name WindowName, usedPct float64, resetsAt time.Time) (Window, error) {
	if name == "" {
		return Window{}, fmt.Errorf("window name is empty: %w", usageerrors.ErrInvalidUsage)
	}
	if math.IsNaN(usedPct) || math.IsInf(usedPct, 0) {
		return Window{}, fmt.Errorf("window %q has no usable percentage: %w", name, usageerrors.ErrInvalidUsage)
	}
	return Window{name: name, usedPct: math.Min(math.Max(usedPct, 0), maxPct), resetsAt: resetsAt}, nil
}

// Name is the window identifier.
func (w Window) Name() WindowName { return w.name }

// UsedPct is the consumed share of the window, between 0 and 100.
func (w Window) UsedPct() float64 { return w.usedPct }

// ResetsAt is when the window renews; the zero time when unknown.
func (w Window) ResetsAt() time.Time { return w.resetsAt }

// Level is normal under 70 %, warn from 70 % and critical from 90 %.
func (w Window) Level() Level {
	switch {
	case w.usedPct >= criticalFromPct:
		return LevelCritical
	case w.usedPct >= warnFromPct:
		return LevelWarn
	default:
		return LevelNormal
	}
}
