package opener

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// FileImageReader reads the image files of the disk.
type FileImageReader struct{}

// NewFileImageReader builds the reader.
func NewFileImageReader() *FileImageReader { return &FileImageReader{} }

// ReadImage returns the bytes of the regular file at path, following symlinks, up to max bytes.
func (*FileImageReader) ReadImage(ctx context.Context, path string, max int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, werrors.ErrOpenPathMissing
		}
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer func() { _ = file.Close() }() // read-only handle: a close failure loses nothing
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat image: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, werrors.ErrOpenPathMissing
	}
	if info.Size() > max {
		return nil, werrors.ErrOpenPathTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	if int64(len(data)) > max {
		return nil, werrors.ErrOpenPathTooLarge
	}
	// The extension is the name of the path, which may be a symlink to anything: the content has to be an image too.
	if !strings.HasPrefix(http.DetectContentType(data), "image/") {
		return nil, werrors.ErrOpenPathBlocked
	}
	return data, nil
}
