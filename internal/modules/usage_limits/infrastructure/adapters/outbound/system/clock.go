// Package system holds the adapters of the usage_limits module that wrap the operating system.
package system

import (
	"time"

	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
)

// Clock reads the wall clock.
type Clock struct{}

var _ outbound.ClockPort = Clock{}

// Now returns the current time.
func (Clock) Now() time.Time { return time.Now() }
