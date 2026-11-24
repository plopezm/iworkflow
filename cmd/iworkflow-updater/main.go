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
		Relaunch:   func(path string) error { return desktop.StartDetached(path) },
		Log:        func(message string) { logger.Info(message) },
	})
	if err != nil {
		logger.Error("update failed", "error", err)
		return 1
	}
	if err = desktop.CleanupPreviousInstall(*target); err != nil {
		logger.Warn("could not remove the previous install", "error", err)
	}
	logger.Info("update applied")
	return 0
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
