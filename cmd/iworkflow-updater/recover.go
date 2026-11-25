package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"iworkflow/internal/desktop"
)

// parentExitTimeout bounds how long recovery waits for the old app to be gone before it reopens it.
const parentExitTimeout = 10 * time.Second

// recovery brings the app back after a failed update. Every side effect is injected so it can be tested.
type recovery struct {
	notify   func(message string)
	waitExit func(ctx context.Context, pid int) error
	exists   func(path string) error
	start    func(dir string, exe string) error
	logPath  string
	timeout  time.Duration
}

func newRecovery() recovery {
	return recovery{
		notify:   notifyFailure,
		waitExit: desktop.WaitForProcessExit,
		exists:   func(path string) error { _, err := os.Stat(path); return err }, //nolint:gosec // the app path given by the app itself
		start:    func(dir string, exe string) error { return desktop.StartDetachedIn(dir, exe) },
		logPath:  filepath.Join(os.TempDir(), "iworkflow-updater.log"),
		timeout:  parentExitTimeout,
	}
}

// run tells the user the update failed and opens the app again: the app closed to let the update run, so
// without this it would simply be gone. It reopens the app only once the old process has exited, because
// otherwise (for example when waiting for it timed out) a second copy would compete with the running one for
// the server port and the data. The app is reopened before the message, which blocks until it is dismissed.
func (r recovery) run(logger *slog.Logger, exe string, parentPID int, cause error) {
	reopened := r.reopen(logger, exe, parentPID)
	suffix := "The app could not be reopened automatically; open it again."
	if reopened {
		suffix = "The app was opened again."
	}
	r.notify(fmt.Sprintf("The update could not be installed:\n\n%v\n\n%s Details: %s", cause, suffix, r.logPath))
}

func (r recovery) reopen(logger *slog.Logger, exe string, parentPID int) bool {
	if exe == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	if err := r.waitExit(ctx, parentPID); err != nil {
		logger.Warn("the old app is still running, so it is not reopened", "pid", parentPID, "error", err)
		return false
	}
	if err := r.exists(exe); err != nil {
		logger.Warn("the app is not at its install path, so it is not reopened", "path", exe, "error", err)
		return false
	}
	if err := r.start(filepath.Dir(exe), exe); err != nil {
		logger.Error("could not reopen the app", "error", err)
		return false
	}
	return true
}
