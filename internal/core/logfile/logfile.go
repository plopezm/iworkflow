// Package logfile writes the log to a file that rotates by size and keeps its backups gzipped.
package logfile

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	dirMode  os.FileMode = 0o755
	fileMode os.FileMode = 0o600
	gzipExt  string      = ".gz"
)

// Writer is an io.WriteCloser safe for concurrent use. When a write would take the file past
// maxBytes, the file is compressed to <path>.1.gz, older backups shift up (<path>.2.gz, ...) and
// those beyond maxBackups are deleted.
type Writer struct {
	mu         sync.Mutex
	path       string
	maxBytes   int64
	maxBackups int
	file       *os.File
	size       int64
}

// Open creates the folder if needed and opens path for appending. maxBackups is at least 1.
func Open(path string, maxBytes int64, maxBackups int) (*Writer, error) {
	if maxBackups < 1 {
		maxBackups = 1
	}
	writer := &Writer{path: path, maxBytes: maxBytes, maxBackups: maxBackups}
	if err := writer.openFile(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *Writer) openFile() error {
	if err := os.MkdirAll(filepath.Dir(w.path), dirMode); err != nil {
		return fmt.Errorf("create log folder: %w", err)
	}
	file, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close() // the stat error is the one worth reporting
		return fmt.Errorf("stat log file: %w", err)
	}
	w.file, w.size = file, info.Size()
	return nil
}

// Write appends p, rotating first when the file would exceed its limit.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if w.maxBytes > 0 && w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	written, err := w.file.Write(p)
	w.size += int64(written)
	if err != nil {
		return written, fmt.Errorf("write log file: %w", err)
	}
	return written, nil
}

// Close closes the file; later writes fail with os.ErrClosed.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	if err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}

func (w *Writer) backup(index int) string {
	return fmt.Sprintf("%s.%d%s", w.path, index, gzipExt)
}

func (w *Writer) rotate() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	w.file = nil
	if err := os.Remove(w.backup(w.maxBackups)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove oldest log backup: %w", err)
	}
	for index := w.maxBackups - 1; index >= 1; index-- {
		if err := os.Rename(w.backup(index), w.backup(index+1)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("shift log backup: %w", err)
		}
	}
	if err := compress(w.path, w.backup(1)); err != nil {
		return err
	}
	if err := os.Remove(w.path); err != nil {
		return fmt.Errorf("remove rotated log file: %w", err)
	}
	return w.openFile()
}

// compress gzips src into dst through a temporary file so a crash never leaves a partial backup.
func compress(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open log file to compress: %w", err)
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("create log backup: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp) // best effort cleanup; the original error is returned
		}
	}()
	zw := gzip.NewWriter(out)
	if _, err = io.Copy(zw, in); err != nil {
		_ = out.Close() // the copy error is the one worth reporting
		return fmt.Errorf("compress log file: %w", err)
	}
	if err = zw.Close(); err != nil {
		_ = out.Close() // the gzip error is the one worth reporting
		return fmt.Errorf("finish log backup: %w", err)
	}
	if err = out.Close(); err != nil {
		return fmt.Errorf("close log backup: %w", err)
	}
	if err = os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("publish log backup: %w", err)
	}
	return nil
}
