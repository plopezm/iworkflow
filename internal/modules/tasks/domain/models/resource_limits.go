//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"fmt"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// ResourceLimits are the resource limits of one agent execution. A nil field leaves that resource
// without a limit.
type ResourceLimits struct {
	// MemoryMB is the RAM cap in MiB.
	MemoryMB *uint32
	// CPUPercent is the CPU cap as a percentage of one core (100 = one full core).
	CPUPercent *uint32
}

// IsUnlimited reports whether there is no limit to apply.
func (l ResourceLimits) IsUnlimited() bool {
	return l.MemoryMB == nil && l.CPUPercent == nil
}

// Or combines two limit sets: each resource defined in l wins over the fallback one.
func (l ResourceLimits) Or(fallback ResourceLimits) ResourceLimits {
	return ResourceLimits{
		MemoryMB:   firstSet(l.MemoryMB, fallback.MemoryMB),
		CPUPercent: firstSet(l.CPUPercent, fallback.CPUPercent),
	}
}

func firstSet(value *uint32, fallback *uint32) *uint32 {
	if value != nil {
		return value
	}
	return fallback
}

// Validated rejects limits set to zero, which make no sense for cgroups.
func (l ResourceLimits) Validated() (ResourceLimits, error) {
	if (l.MemoryMB != nil && *l.MemoryMB == 0) || (l.CPUPercent != nil && *l.CPUPercent == 0) {
		return ResourceLimits{}, werrors.InvalidLimits()
	}
	return l, nil
}

// SystemdProperties returns the `systemd-run -p` properties equivalent to the defined limits.
func (l ResourceLimits) SystemdProperties() []string {
	var properties []string
	if l.MemoryMB != nil {
		properties = append(properties, fmt.Sprintf("MemoryMax=%dM", *l.MemoryMB))
	}
	if l.CPUPercent != nil {
		properties = append(properties, fmt.Sprintf("CPUQuota=%d%%", *l.CPUPercent))
	}
	return properties
}
