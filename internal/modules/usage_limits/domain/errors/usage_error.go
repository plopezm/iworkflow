// Package errors holds the business errors of the usage_limits module.
package errors

import "errors"

// Sentinel errors. Callers test them with errors.Is.
var (
	// ErrNotConfigured means the provider has no credentials or data on this machine: it is hidden, not an error.
	ErrNotConfigured = errors.New("provider not configured")
	// ErrUnavailable means the provider could not be read right now.
	ErrUnavailable = errors.New("provider usage unavailable")
	// ErrInvalidUsage means the provider answered data that breaks an invariant.
	ErrInvalidUsage = errors.New("invalid usage data")
)
