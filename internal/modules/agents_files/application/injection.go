// Package application holds the agents_files use cases. Each use case is a struct with one Execute
// method and receives its outbound ports through the constructor.
package application

import (
	"time"

	"iworkflow/internal/modules/agents_files/domain/models"
)

// Clock returns the current time. It is injected so that timestamps are testable.
type Clock func() time.Time

// IDGenerator returns a new identifier. It is injected so that identifiers are testable.
type IDGenerator func() (models.AgentsFileID, error)
