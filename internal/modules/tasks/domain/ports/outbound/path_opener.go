package outbound

import "context"

// PathOpener opens a file or a folder of the disk with the program that the operating system associates with it.
type PathOpener interface {
	// Open opens an absolute path. It returns errors.ErrOpenPathMissing when the path does not exist.
	Open(ctx context.Context, path string) error
}
