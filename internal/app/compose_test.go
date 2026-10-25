package app

import (
	"testing"
	"time"

	"iworkflow/internal/core/config"
)

// NewTestConfig is the configuration of a server whose folders are temporary. The runtime binaries are names only:
// the tests compose the modules but never start an agent. It is exported for the external tests of the package.
func NewTestConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		HomeDir:            t.TempDir(),
		GentleShellBin:     "gentle-shell",
		PiBin:              "pi",
		ClaudeBin:          "claude",
		CursorBin:          "agent",
		CursorSessionsDir:  t.TempDir(),
		WorktreesDir:       t.TempDir(),
		AttachmentsDir:     t.TempDir(),
		AgentsFilesDir:     t.TempDir(),
		GentlePiConfigHome: t.TempDir(),
		PiAgentsDir:        t.TempDir(),
	}
}

// LoopSweeps reports how many sweeps the graph loop and the resume loop have started. The tests read it to see that
// the loops run.
func (a *App) LoopSweeps() (graph int64, resume int64) {
	return a.loops.graphSweeps.Load(), a.loops.resumeSweeps.Load()
}

// SetLoopIntervals replaces the intervals of the background loops before Start, so that a test sees many sweeps fast.
func (a *App) SetLoopIntervals(graph time.Duration, resume time.Duration) {
	a.loops.graphTick = graph
	a.loops.resumeTick = resume
}
