// Package probe answers the launch preparation of a task with what the tools around its related projects offer.
// It only inspects the file system and the PATH: it never runs the tools.
package probe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// engramExecutable is the engram command the agents use for their memory.
const engramExecutable = "engram"

// engramHomeFolder is the folder engram keeps its data in, under the home folder of the user.
const engramHomeFolder = ".engram"

// codegraphFolder is the index folder of a project that has codegraph initialized.
const codegraphFolder = ".codegraph"

// RelatedProjectProbe reads the codegraph index of a project and the engram installation of the machine.
type RelatedProjectProbe struct{}

// NewRelatedProjectProbe builds the probe. It holds no state.
func NewRelatedProjectProbe() *RelatedProjectProbe {
	return &RelatedProjectProbe{}
}

// HasCodegraph reports whether the project has a codegraph folder. An empty path never has one.
func (p *RelatedProjectProbe) HasCodegraph(_ context.Context, projectPath string) bool {
	if projectPath == "" {
		return false
	}
	return isDirectory(filepath.Join(projectPath, codegraphFolder))
}

// EngramAvailable reports whether the engram executable is on the PATH or its home folder exists.
func (p *RelatedProjectProbe) EngramAvailable(_ context.Context) bool {
	if _, err := exec.LookPath(engramExecutable); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return isDirectory(filepath.Join(home, engramHomeFolder))
}

// isDirectory reports whether path exists and is a folder. Any error counts as false.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
