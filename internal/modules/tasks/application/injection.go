// Package application holds the tasks use cases. Each use case is a struct with one Execute method and
// receives its outbound ports through the constructor. Errors of the ports pass through unchanged.
package application

import (
	"time"

	"github.com/google/uuid"
)

// Clock returns the current time. It is injected so that timestamps are testable.
type Clock func() time.Time

// UUIDGenerator returns a new random UUID. It is injected so that identifiers are testable.
type UUIDGenerator func() (uuid.UUID, error)
