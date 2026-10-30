package outbound

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// DirectoryReaderPort lists the filesystem. Every method is confined to the root it receives.
type DirectoryReaderPort interface {
	// List lists one directory level of relative (empty means the root).
	List(ctx context.Context, root string, relative string) (models.DirectoryContents, error)
}

// FileStorePort reads and overwrites text files. Every method is confined to the root it receives.
type FileStorePort interface {
	// Read returns the UTF-8 text file at relative.
	Read(ctx context.Context, root string, relative string) (models.ProjectFile, error)
	// Write overwrites the existing text file at relative.
	Write(ctx context.Context, root string, relative string, content string) error
}
