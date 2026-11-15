package outbound

import "context"

// PathOpener opens a file or a folder of the disk with the program that the operating system associates with it.
type PathOpener interface {
	// Open opens an absolute path. It returns errors.ErrOpenPathMissing when the path does not exist.
	Open(ctx context.Context, path string) error
}

// ImageReader reads an image file of the disk for a preview in the page.
type ImageReader interface {
	// ReadImage returns the bytes of the regular file at the absolute path. It returns errors.ErrOpenPathMissing when
	// the path does not exist or is not a file, and errors.ErrOpenPathTooLarge when it is bigger than max bytes.
	ReadImage(ctx context.Context, path string, max int64) ([]byte, error)
}
