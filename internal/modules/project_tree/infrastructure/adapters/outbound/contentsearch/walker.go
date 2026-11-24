package contentsearch

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

const (
	// maxWalkFileBytes skips big files, as ripgrep's --max-filesize does.
	maxWalkFileBytes int64 = 1 << 20
	// binarySniffBytes is how much of a file is checked for a NUL byte.
	binarySniffBytes int = 8000
)

// skippedDirs are never entered by the walker, which does not read .gitignore.
var skippedDirs = map[string]struct{}{".git": {}, "node_modules": {}, "dist": {}}

// WalkSearcher is the pure Go fallback used when ripgrep is not installed.
type WalkSearcher struct{}

var _ outbound.ContentSearchPort = WalkSearcher{}

// NewWalkSearcher builds the fallback searcher.
func NewWalkSearcher() WalkSearcher { return WalkSearcher{} }

// Search walks root in path order and scans every text file.
func (WalkSearcher) Search(ctx context.Context, root string, query models.ContentQuery) (models.ContentResult, error) {
	pattern, err := query.Pattern()
	if err != nil {
		return models.ContentResult{}, fmt.Errorf("compile query: %w", err)
	}
	var result models.ContentResult
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are skipped, as ripgrep does
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("search cancelled: %w", ctxErr)
		}
		if entry.IsDir() {
			if _, skip := skippedDirs[entry.Name()]; skip && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		file, ok := scanFile(path, pattern.FindAllStringIndex)
		if !ok {
			return nil
		}
		if len(result.Files) >= models.MaxContentFiles {
			result.Truncated = true
			return filepath.SkipAll
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil //nolint:nilerr // a path outside root cannot happen under WalkDir(root)
		}
		file.Path = filepath.ToSlash(relative)
		result.Files = append(result.Files, file)
		return nil
	})
	if walkErr != nil {
		return models.ContentResult{}, fmt.Errorf("walk project: %w", walkErr)
	}
	return result, nil
}

// scanFile returns the matches of one file, and false when it has none, is binary, too big or unreadable.
func scanFile(path string, find func(string, int) [][]int) (models.ContentFile, bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxWalkFileBytes {
		return models.ContentFile{}, false
	}
	handle, err := os.Open(path) //nolint:gosec // path comes from WalkDir under the project root
	if err != nil {
		return models.ContentFile{}, false
	}
	defer func() { _ = handle.Close() }()
	reader := bufio.NewReaderSize(handle, 64*1024)
	head, _ := reader.Peek(binarySniffBytes)
	if bytes.IndexByte(head, 0) >= 0 {
		return models.ContentFile{}, false
	}
	var file models.ContentFile
	for lineNumber := 1; ; lineNumber++ {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, "\r\n") // like ripgrep, match without the terminator so `$` works
		if raw != "" {
			for _, loc := range find(line, -1) {
				file.Total++
				if len(file.Matches) < models.MaxMatchesPerFile {
					file.Matches = append(file.Matches, models.ContentMatch{
						Line:    lineNumber,
						Col:     charCol(line, loc[0]),
						Len:     len([]rune(line[loc[0]:loc[1]])),
						Snippet: snippetOf(line),
					})
				}
			}
		}
		if readErr != nil {
			break
		}
	}
	return file, file.Total > 0
}
