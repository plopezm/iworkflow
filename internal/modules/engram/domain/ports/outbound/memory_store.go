// Package outbound holds the ports the engram application layer depends on.
package outbound

import (
	"context"

	"iworkflow/internal/modules/engram/domain/models"
)

// MemoryStore reads what Engram remembers. Implementations must be read-only and must return
// errors wrapping domain/errors.ErrUnavailable when Engram cannot answer.
type MemoryStore interface {
	// Projects lists the Engram projects with their directories.
	Projects(ctx context.Context) ([]models.MemoryProject, error)
	// Recent lists up to limit observations of a project, without their content.
	Recent(ctx context.Context, project string, limit int) ([]models.Memory, error)
	// Observation reads one observation with its content. It returns an error wrapping domain/errors.ErrNotFound when
	// the observation does not exist.
	Observation(ctx context.Context, id int) (models.MemoryDetail, error)
}
