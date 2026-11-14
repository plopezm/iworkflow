// Package opener opens paths of the disk with the program of the operating system.
package opener

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// OSOpener opens a path with explorer (Windows), open (macOS) or xdg-open (Linux and the rest).
type OSOpener struct {
	goos string
}

var _ outbound.PathOpener = OSOpener{}

// NewOSOpener returns the opener of the running operating system.
func NewOSOpener() OSOpener {
	return OSOpener{goos: runtime.GOOS}
}

// command is the program and arguments that open path in goos. The path is always absolute, so it never reads as an
// option.
func command(goos string, path string) (string, []string) {
	switch goos {
	case "windows":
		return "explorer", []string{path}
	case "darwin":
		return "open", []string{path}
	default:
		return "xdg-open", []string{path}
	}
}

// Open starts the opener and does not wait for the program it launches.
func (o OSOpener) Open(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return werrors.ErrOpenPathMissing
		}
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	name, args := command(o.goos, path)
	// The program must outlive the request, so only the values of its context are kept.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release %s: %w", name, err)
	}
	return nil
}
