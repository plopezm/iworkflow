//go:build gtk3 || !linux

package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

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
	view := newProgressView(s.app)
	flow, err := desktop.NewFlow(desktop.Config{
		Strategy:   strategy,
		Installer:  installer,
		AppVersion: version,
		GHToken:    os.Getenv("GH_TOKEN"),
		Ask:        s.ask,
		Progress:   view.update,
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
	view.cancel = flow.CancelUpdate
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
	if err = desktop.StartDetachedIn(scratch, copied,
		"-archive", args.Archive, "-sha256", args.SHA256, "-target", args.Target,
		"-exe", args.Executable, "-staging", args.Staging, "-pid", strconv.Itoa(args.ParentPID), "-cleanup-dir", scratch); err != nil {
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

// progressView is the foreground window of an automatic update: it shows the download and the install until
// the app quits, or closes when the update ends without installing.
//
// Closing the window while it downloads cancels the update (cancel); while it installs the close is refused.
// The close hook runs on the GUI thread, which update() may be waiting on while it holds mu, so the hook only
// touches atomics and never takes mu.
type progressView struct {
	app    *application.App
	cancel func() // set before the first update; ends the download in progress

	mu      sync.Mutex
	window  *application.WebviewWindow
	lastAt  time.Time
	lastPha desktop.ProgressPhase

	installing atomic.Bool // the install cannot be interrupted, so its window cannot be closed
	programmed atomic.Bool // the window is being closed by update(), not by the user
	userClosed atomic.Bool // the user closed the window and cancelled the update
}

func newProgressView(app *application.App) *progressView {
	return &progressView{app: app}
}

// progressRefresh limits how often the window content is rewritten while bytes arrive.
const progressRefresh = 200 * time.Millisecond

func (v *progressView) update(p desktop.Progress) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.installing.Store(p.Phase == desktop.PhaseInstalling)
	if v.userClosed.Load() {
		// The window is gone and the download is being cancelled: show nothing until the update ends.
		v.window = nil
		if p.Phase == desktop.PhaseEnded {
			v.userClosed.Store(false)
		}
		return
	}
	if p.Phase == desktop.PhaseEnded {
		if v.window != nil {
			window := v.window
			v.window = nil
			v.programmed.Store(true)
			application.InvokeSync(window.Close)
			v.programmed.Store(false)
		}
		return
	}
	now := time.Now()
	if v.window != nil && p.Phase == v.lastPha && now.Sub(v.lastAt) < progressRefresh {
		return
	}
	v.lastAt, v.lastPha = now, p.Phase
	page := progressPage(p)
	application.InvokeSync(func() {
		if v.window == nil {
			v.window = v.app.Window.NewWithOptions(application.WebviewWindowOptions{
				Title:         appTitle,
				Width:         460,
				Height:        180,
				AlwaysOnTop:   true,
				DisableResize: true,
				HTML:          page,
			})
			v.window.RegisterHook(events.Common.WindowClosing, v.onClosing)
			v.window.Center()
			v.window.Show()
			return
		}
		v.window.SetHTML(page)
	})
}

// onClosing handles the close button of the progress window.
func (v *progressView) onClosing(event *application.WindowEvent) {
	if v.programmed.Load() {
		return
	}
	if v.installing.Load() {
		event.Cancel()
		return
	}
	v.userClosed.Store(true)
	if v.cancel != nil {
		v.cancel()
	}
}

// leaveInstallFolder moves the working directory of the app out of the install folder. Processes started by
// the app, such as WebView2, inherit it, and on Windows a process working inside a folder keeps it from being
// renamed by the update. The home folder is what the server already uses.
func leaveInstallFolder(logger interface{ Warn(msg string, args ...any) }) {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Warn("could not find the home folder to leave the install folder", "error", err)
		return
	}
	if err = os.Chdir(home); err != nil {
		logger.Warn("could not leave the install folder", "error", err)
	}
}

// progressPage renders the status of an update as a small self-contained page.
func progressPage(p desktop.Progress) string {
	title := fmt.Sprintf("Downloading iworkflow %s", html.EscapeString(p.Version))
	detail := "Starting the download…"
	bar := `<progress></progress>`
	switch {
	case p.Phase == desktop.PhaseInstalling:
		title = fmt.Sprintf("Installing iworkflow %s", html.EscapeString(p.Version))
		detail = "The app restarts by itself when it finishes. Please wait."
	case p.Total > 0:
		percent := p.Done * 100 / p.Total
		bar = fmt.Sprintf(`<progress max="100" value="%d"></progress>`, percent)
		detail = fmt.Sprintf("%d%% (%d of %d MiB)", percent, p.Done>>20, p.Total>>20)
	case p.Done > 0:
		detail = fmt.Sprintf("%d MiB downloaded", p.Done>>20)
	}
	return `<!doctype html><meta charset="utf-8"><body style="font-family:sans-serif;margin:24px;background:#111;color:#eee">` +
		`<h3 style="margin:0 0 16px">` + title + `</h3>` + bar +
		`<p style="color:#aaa">` + detail + `</p>` +
		`<style>progress{width:100%;height:14px}</style></body>`
}
