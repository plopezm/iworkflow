// Package system reads the application log file from disk.
package system

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"iworkflow/internal/modules/logs/domain/models"
	"iworkflow/internal/modules/logs/domain/ports/outbound"
)

// Bounds of one read. The current file is read through a byte window (maxReadBytes) because it can be
// seeked; a gzip backup cannot, so it is streamed in full and only the last lines are kept. A single
// line longer than maxLineBytes makes the backup unreadable (the current file has no such limit: it is
// split from the window as text).
const (
	maxReadBytes     int64 = 1 << 20
	maxLineBytes     int   = 1 << 20
	scannerInitBytes int   = 64 << 10
)

// FileLogReader reads the log file and, when the current file is short, its newest gzip backup.
type FileLogReader struct {
	path string
}

var _ outbound.LogReaderPort = (*FileLogReader)(nil)

// NewFileLogReader builds a reader of the log file at path (backups are path + ".1.gz").
func NewFileLogReader(path string) *FileLogReader {
	return &FileLogReader{path: path}
}

// Tail returns the most recent maxLines lines, oldest first. A missing file is an empty tail.
func (r *FileLogReader) Tail(ctx context.Context, maxLines int) (models.LogTail, error) {
	if err := ctx.Err(); err != nil {
		return models.LogTail{}, fmt.Errorf("read log: %w", err)
	}
	lines, partial, err := tailOfFile(r.path, maxLines)
	if err != nil {
		return models.LogTail{}, err
	}
	truncated := partial
	// The backup completes a short file. When the window already cut the file, the lines before it are
	// lost for good, so joining the backup would leave a silent gap: it is not read.
	if len(lines) < maxLines && !partial {
		older, olderErr := tailOfGzip(r.path+".1.gz", maxLines-len(lines))
		if olderErr != nil {
			// The backup is a best-effort supplement (it can be half written by a rotation or corrupt):
			// the current file was read, so show it and say older lines are missing.
			slog.Debug("log backup not readable", "path", r.path+".1.gz", "error", olderErr)
			truncated = true
		} else {
			lines = append(older.lines, lines...)
			truncated = older.truncated
		}
	}
	return models.LogTail{Lines: lines, Truncated: truncated}, nil
}

type tailResult struct {
	lines     []string
	truncated bool
}

// tailOfFile reads the last bytes of the file. The second result is true when the file holds more
// than the lines returned.
func tailOfFile(path string, maxLines int) ([]string, bool, error) {
	file, err := os.Open(path) //nolint:gosec // The path is the application log file, fixed at wiring.
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("open log: %w", err)
	}
	defer file.Close() //nolint:errcheck // Read-only file: nothing to recover from a failed close.
	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("stat log: %w", err)
	}
	offset := max(info.Size()-maxReadBytes, 0)
	// From the byte before the window: when it is a line break the first line of the window is whole.
	start := max(offset-1, 0)
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, false, fmt.Errorf("seek log: %w", err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, false, fmt.Errorf("read log: %w", err)
	}
	text := string(data)
	if offset > 0 {
		if text[:1] == "\n" {
			text = text[1:]
		} else if _, rest, found := strings.Cut(text, "\n"); found {
			text = rest // the first line starts before the window
		} else {
			text = ""
		}
	}
	lines := splitLines(text)
	truncated := offset > 0
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		truncated = true
	}
	return lines, truncated, nil
}

// tailOfGzip reads the last maxLines lines of a gzip backup. A missing backup is empty.
func tailOfGzip(path string, maxLines int) (tailResult, error) {
	file, err := os.Open(path) //nolint:gosec // The path is the application log backup, fixed at wiring.
	if errors.Is(err, fs.ErrNotExist) {
		return tailResult{}, nil
	}
	if err != nil {
		return tailResult{}, fmt.Errorf("open log backup: %w", err)
	}
	defer file.Close() //nolint:errcheck // Read-only file: nothing to recover from a failed close.
	zr, err := gzip.NewReader(file)
	if err != nil {
		return tailResult{}, fmt.Errorf("open log backup: %w", err)
	}
	ring := make([]string, 0, maxLines)
	total := 0
	scanner := bufio.NewScanner(zr)
	scanner.Buffer(make([]byte, 0, scannerInitBytes), maxLineBytes)
	for scanner.Scan() {
		total++
		if len(ring) == maxLines {
			ring = ring[1:]
		}
		ring = append(ring, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return tailResult{}, fmt.Errorf("read log backup: %w", err)
	}
	return tailResult{lines: ring, truncated: total > len(ring)}, nil
}

func splitLines(text string) []string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
