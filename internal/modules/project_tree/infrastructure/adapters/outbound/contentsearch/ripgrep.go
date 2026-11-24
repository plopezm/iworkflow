package contentsearch

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// maxFileSize keeps ripgrep and the walker away from huge generated files.
const maxFileSize string = "1M"

// RipgrepSearcher searches with `rg --json`: fast, and it honours .gitignore.
type RipgrepSearcher struct {
	binary string
}

var _ outbound.ContentSearchPort = (*RipgrepSearcher)(nil)

// NewRipgrepSearcher uses the rg executable at binary.
func NewRipgrepSearcher(binary string) *RipgrepSearcher {
	return &RipgrepSearcher{binary: binary}
}

// rgEvent is the part of a ripgrep JSON line this adapter reads.
type rgEvent struct {
	Type string `json:"type"`
	Data struct {
		Path       rgText `json:"path"`
		Lines      rgText `json:"lines"`
		LineNumber int    `json:"line_number"`
		Submatches []struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"submatches"`
	} `json:"data"`
}

// rgText is ripgrep's text wrapper; non-UTF-8 content is sent as base64 under another key and ignored.
type rgText struct {
	Text string `json:"text"`
}

// Search runs ripgrep in root and folds its events into a result.
func (s *RipgrepSearcher) Search(ctx context.Context, root string, query models.ContentQuery) (models.ContentResult, error) {
	pattern, err := query.Pattern()
	if err != nil {
		return models.ContentResult{}, fmt.Errorf("compile query: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	//nolint:gosec // The binary comes from exec.LookPath; the pattern is passed as a single argument after -e.
	cmd := exec.CommandContext(runCtx, s.binary, "--json", "--hidden", "--case-sensitive", "--max-filesize", maxFileSize,
		"--glob", "!.git", "--sort", "path", "-e", pattern.String(), "--", ".")
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return models.ContentResult{}, fmt.Errorf("rg stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return models.ContentResult{}, fmt.Errorf("start rg: %w", err)
	}
	result, readErr := collectRipgrep(stdout, cancel)
	waitErr := cmd.Wait()
	if readErr != nil {
		return models.ContentResult{}, readErr
	}
	if err := ctx.Err(); err != nil {
		return models.ContentResult{}, fmt.Errorf("search cancelled: %w", err)
	}
	var exitErr *exec.ExitError
	// Exit 1 means "no matches"; a kill after the file limit is expected too.
	expected := errors.As(waitErr, &exitErr) && (exitErr.ExitCode() == 1 || result.Truncated)
	if waitErr != nil && !expected {
		return models.ContentResult{}, fmt.Errorf("rg failed: %w", waitErr)
	}
	return result, nil
}

// collectRipgrep reads the JSON lines until EOF, or until the file limit is exceeded, when it calls stop.
func collectRipgrep(stdout interface{ Read([]byte) (int, error) }, stop context.CancelFunc) (models.ContentResult, error) {
	var result models.ContentResult
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var current *models.ContentFile
	for scanner.Scan() {
		var event rgEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return models.ContentResult{}, fmt.Errorf("decode rg output: %w", err)
		}
		if event.Type != "match" || event.Data.Path.Text == "" {
			continue
		}
		path := strings.TrimPrefix(event.Data.Path.Text, "./")
		if current == nil || current.Path != path {
			if len(result.Files) >= models.MaxContentFiles {
				result.Truncated = true
				stop()
				break
			}
			result.Files = append(result.Files, models.ContentFile{Path: path})
			current = &result.Files[len(result.Files)-1]
		}
		line := event.Data.Lines.Text
		for _, sub := range event.Data.Submatches {
			current.Total++
			if len(current.Matches) < models.MaxMatchesPerFile {
				current.Matches = append(current.Matches, models.ContentMatch{
					Line:    event.Data.LineNumber,
					Col:     charCol(line, sub.Start),
					Len:     len([]rune(line[min(sub.Start, len(line)):min(sub.End, len(line))])),
					Snippet: snippetOf(line),
				})
			}
		}
	}
	if err := scanner.Err(); err != nil && !result.Truncated {
		return models.ContentResult{}, fmt.Errorf("read rg output: %w", err)
	}
	return result, nil
}
