package outbound

import (
	"context"

	"iworkflow/internal/modules/agents_files/domain/models"
)

// AgentsFileStorage keeps the content of the AGENTS.md files on disk.
type AgentsFileStorage interface {
	// Store writes the content and returns the absolute path where it was left.
	Store(ctx context.Context, id models.AgentsFileID, content []byte) (string, error)
	Read(ctx context.Context, path string) ([]byte, error)
	// Remove deletes the file; a file that is already gone is not an error.
	Remove(ctx context.Context, path string) error
}
