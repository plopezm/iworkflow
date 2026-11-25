// Command iworkflow-updater replaces the installed desktop app with a downloaded, already verified update.
// The desktop shell starts it as an independent process and exits; it waits for the shell to end, swaps the
// install folder (rolling back if the swap fails) and starts the new version.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"

	"iworkflow/internal/desktop"
)

func main() {
	os.Exit(run())
}

func run() int {
	archive := flag.String("archive", "", "downloaded update archive (tar.gz)")
	sum := flag.String("sha256", "", "expected sha256 of the archive")
	target := flag.String("target", "", "install folder to replace")
	exe := flag.String("exe", "", "running executable inside the install folder")
	pid := flag.Int("pid", 0, "process ID of the app to wait for")
	staging := flag.String("staging", "", "folder the app already extracted the update into")
	cleanup := flag.String("cleanup-dir", "", "temporary folder holding this binary, removed at the end")
	flag.Parse()

	logger, closeLog := newLogger()
	defer closeLog()
	if *cleanup != "" {
		defer func() {
			// On Windows a running binary cannot be removed: the leftover temp folder is harmless.
			if err := os.RemoveAll(*cleanup); err != nil {
				logger.Warn("could not remove the helper folder", "error", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := desktop.ApplyUpdate(ctx, desktop.ApplyOptions{
		Archive:    *archive,
		SHA256:     *sum,
		Target:     *target,
		Executable: *exe,
		ParentPID:  *pid,
		Staging:    *staging,
		// The new app starts in its own folder, never in the helper folder that is removed right after.
		Relaunch: func(path string) error { return desktop.StartDetachedIn(filepath.Dir(path), path) },
		Log:      func(message string) { logger.Info(message) },
	})
	if err != nil {
		logger.Error("update failed", "error", err)
		recoverApp(logger, *exe, err)
		return 1
	}
	if err = desktop.CleanupPreviousInstall(*target); err != nil {
		logger.Warn("could not remove the previous install", "error", err)
	}
	logger.Info("update applied")
	return 0
}

// recoverApp tells the user the update failed and opens the app again: the app already closed to let the
// update run, so without this it would simply be gone. The log file path is part of the message.
func recoverApp(logger *slog.Logger, exe string, cause error) {
	notifyFailure(fmt.Sprintf("The update could not be installed:\n\n%v\n\nThe app opens again. Details: %s",
		cause, filepath.Join(os.TempDir(), "iworkflow-updater.log")))
	if exe == "" {
		return
	}
	if _, err := os.Stat(exe); err != nil {
		logger.Warn("the app is not at its install path, so it is not reopened", "path", exe, "error", err)
		return
	}
	if err := desktop.StartDetachedIn(filepath.Dir(exe), exe); err != nil {
		logger.Error("could not reopen the app", "error", err)
	}
}

// newLogger writes to a file in the temp folder: the helper has no console to report to.
func newLogger() (*slog.Logger, func()) {
	file, err := os.OpenFile(filepath.Join(os.TempDir(), "iworkflow-updater.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}
	}
	return slog.New(slog.NewTextHandler(file, nil)), func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, closeErr)
		}
	}
}
