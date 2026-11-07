// Package system holds the real clock and identifier adapters of the task_hooks module.
package system

import (
	"time"

	"github.com/google/uuid"
)

// Clock is the wall clock.
type Clock struct{}

// NewClock builds the wall clock.
func NewClock() Clock {
	return Clock{}
}

// Now returns the current time.
func (Clock) Now() time.Time {
	return time.Now()
}

// UUIDGenerator creates the identifiers of hooks and runs as random version 4 UUIDs.
type UUIDGenerator struct{}

// NewUUIDGenerator builds the generator.
func NewUUIDGenerator() UUIDGenerator {
	return UUIDGenerator{}
}

// NewID returns a new random UUID in its canonical text form.
func (UUIDGenerator) NewID() string {
	return uuid.NewString()
}
