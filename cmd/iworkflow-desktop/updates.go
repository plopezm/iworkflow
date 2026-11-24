//go:build gtk3 || !linux

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"

	"iworkflow/internal/desktop"
)

// version is stamped at build time (-ldflags "-X main.version=..."); "dev" never offers an update.
var version = "dev"

// newUpdateFlow builds the update flow of the app, or nil when updates are off for this run. A run with
// IWF_SERVER_DIR set is a development run (make wails-dev), never a packaged app.
func (s *shell) newUpdateFlow() *desktop.Flow {
	strategy := desktop.UpdateStrategy(desktop.PolicyInput{
		GOOS:     runtime.GOOS,
		Packaged: os.Getenv("IWF_SERVER_DIR") == "",
		Getenv:   os.Getenv,
	})
	if strategy == desktop.StrategyOff {
		return nil
	}
	installer := s.newInstaller()
	flow, err := desktop.NewFlow(desktop.Config{
		Strategy:   strategy,
		Installer:  installer,
		AppVersion: version,
		GHToken:    os.Getenv("GH_TOKEN"),
		Ask:        s.ask,
		OpenExternal: func(_ context.Context, url string) error {
			if err := s.app.Browser.OpenURL(url); err != nil {
				return fmt.Errorf("opening %s: %w", url, err)
			}
			return nil
		},
		// The installer cannot replace files while the server runs.
		BeforeInstall: func(context.Context) error {
			s.super.Stop()
			return nil
		},
		Recover: s.app.Quit,
		Log:     func(message string) { s.logger.Info("update", "message", message) },
	})
	if err != nil {
		s.logger.Warn("updates disabled", "error", err)
		return nil
	}
	return flow
}

// newInstaller builds the archive installer for this install, or nil (the flow then only notifies) when the
// location of the app cannot be worked out.
func (s *shell) newInstaller() desktop.Installer {
	executable, err := os.Executable()
	if err != nil {
		s.logger.Warn("self-update unavailable: executable path", "error", err)
		return nil
	}
	target, err := desktop.InstallTargetFromExecutable(executable, runtime.GOOS)
	if err != nil {
		s.logger.Warn("self-update unavailable: install folder", "error", err)
		return nil
	}
	installer, err := desktop.NewArchiveInstaller(desktop.ArchiveConfig{
		AppVersion: version,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		GHToken:    os.Getenv("GH_TOKEN"),
		Target:     target,
		Executable: executable,
		StartHelper: func(args desktop.HelperArgs) error {
			args.ParentPID = os.Getpid()
			return startHelper(executable, args)
		},
		// The helper waits for this process to end before it replaces the files.
		Exit: func() { s.quit() },
		Log:  func(message string) { s.logger.Info("update", "message", message) },
	})
	if err != nil {
		s.logger.Warn("self-update unavailable", "error", err)
		return nil
	}
	return installer
}

// ask shows a dialog and returns the index of the button the user pressed. A question without buttons
// is acknowledged with a single OK.
func (s *shell) ask(ctx context.Context, question desktop.Question) (int, error) {
	buttons := question.Buttons
	if len(buttons) == 0 {
		buttons = []string{"OK"}
	}
	chosen := make(chan int, 1)
	application.InvokeSync(func() {
		var dialog *application.MessageDialog
		if question.Type == desktop.QuestionError {
			dialog = s.app.Dialog.Error()
		} else {
			dialog = s.app.Dialog.Info()
		}
		dialog.SetTitle(appTitle).SetMessage(joinMessage(question))
		for index, label := range buttons {
			button := dialog.AddButton(label).OnClick(func() {
				select {
				case chosen <- index:
				default:
				}
			})
			if index == question.DefaultID {
				dialog.SetDefaultButton(button)
			}
			if index == question.CancelID {
				dialog.SetCancelButton(button)
			}
		}
		dialog.Show()
	})
	select {
	case index := <-chosen:
		return index, nil
	case <-ctx.Done():
		return question.CancelID, fmt.Errorf("waiting for the dialog: %w", ctx.Err())
	}
}

func joinMessage(question desktop.Question) string {
	if question.Detail == "" {
		return question.Message
	}
	return question.Message + "\n\n" + question.Detail
}

// helperName is the updater binary shipped next to the desktop binary.
func helperName() string {
	if runtime.GOOS == "windows" {
		return "iworkflow-updater.exe"
	}
	return "iworkflow-updater"
}

// startHelper copies the updater to a temporary folder (so that it is not inside the folder it replaces, where
// Windows would lock it) and starts it as an independent process. It returns once the process has started.
func startHelper(executable string, args desktop.HelperArgs) error {
	source := filepath.Join(filepath.Dir(executable), helperName())
	scratch, err := os.MkdirTemp("", "iworkflow-updater-*")
	if err != nil {
		return fmt.Errorf("creating the helper folder: %w", err)
	}
	copied := filepath.Join(scratch, helperName())
	if err = copyExecutable(source, copied); err != nil {
		_ = os.RemoveAll(scratch) //nolint:errcheck // best effort: the copy failed already
		return err
	}
	if err = desktop.StartDetached(copied,
		"-archive", args.Archive, "-sha256", args.SHA256, "-target", args.Target,
		"-exe", args.Executable, "-pid", strconv.Itoa(args.ParentPID), "-cleanup-dir", scratch); err != nil {
		_ = os.RemoveAll(scratch) //nolint:errcheck // best effort: the start failed already
		return fmt.Errorf("starting the updater: %w", err)
	}
	return nil
}

func copyExecutable(source string, destination string) (err error) {
	in, err := os.Open(source) //nolint:gosec // the helper shipped next to this binary
	if err != nil {
		return fmt.Errorf("opening the updater %s: %w", source, err)
	}
	defer func() { _ = in.Close() }()                                               //nolint:errcheck // read-only handle
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700) //nolint:gosec // temp copy of the helper
	if err != nil {
		return fmt.Errorf("creating %s: %w", destination, err)
	}
	defer func() {
		if closeErr := out.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("closing %s: %w", destination, closeErr)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("copying the updater: %w", err)
	}
	return nil
}
