// Command iworkflow-desktop is the desktop shell (Wails 3): it starts the iworkflow server as a child
// process and shows it in a window. The server keeps its own network settings, so a phone can still reach
// it; closing the window only hides it and "Quit" stops the server.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"iworkflow/internal/desktop"
)

const appTitle = "iworkflow"

//go:embed icon.png
var icon []byte

type shell struct {
	app    *application.App
	window *application.WebviewWindow
	super  *desktop.Supervisor
	logger *slog.Logger

	mu       sync.Mutex
	quitting bool
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if err := run(logger); err != nil {
		logger.Error("desktop shell failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable path: %w", err)
	}
	sh := &shell{logger: logger}
	sh.super = &desktop.Supervisor{
		Dir:    desktop.ServerDir(os.Getenv, dirOf(exe)),
		Getenv: os.Getenv,
		Logger: logger,
		Exit:   sh.onServerExit,
	}

	// The single-instance lock is taken by application.New, so it comes before the server starts: a second
	// launch must exit without leaving a server behind.
	sh.app = application.New(application.Options{
		Name: appTitle,
		Icon: icon,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "dev.iworkflow.desktop",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { sh.showWindow() },
		},
		OnShutdown: sh.super.Stop,
	})
	// The server starts before the GUI loop so a start failure is reported without opening a window.
	addr, err := sh.super.Start(context.Background())
	if err != nil {
		sh.super.Stop()
		return fmt.Errorf("starting the server: %w\n%s", err, sh.super.LogTail())
	}

	sh.window = sh.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  appTitle,
		Width:  1400,
		Height: 900,
	})
	// The certificate hook must be in place before the first load, so the URL is set once the GUI runs.
	sh.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		application.InvokeSync(func() {
			if err := installWebViewGuards(sh.window.NativeWindow(), addr.Port); err != nil {
				logger.Warn("could not install the web view guards", "error", err)
			}
		})
		sh.trustServer(addr)
		sh.window.SetURL(addr.WindowURL(sh.super.Token()))
	})
	// SIGINT/SIGTERM must stop the server too: route them to a normal quit.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		sh.quit()
	}()
	// The server stays up for remote devices: closing the window only hides it.
	sh.window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if !sh.isQuitting() {
			event.Cancel()
			sh.window.Hide()
		}
	})
	sh.createTray()
	if err = sh.app.Run(); err != nil {
		return fmt.Errorf("running the app: %w", err)
	}
	return nil
}

func (s *shell) createTray() {
	tray := s.app.SystemTray.New()
	tray.SetIcon(icon)
	tray.SetTooltip(appTitle + " (server running)")
	menu := s.app.NewMenu()
	menu.Add("Open iworkflow").OnClick(func(*application.Context) { s.showWindow() })
	menu.AddSeparator()
	menu.Add("Quit and stop server").OnClick(func(*application.Context) { s.quit() })
	tray.SetMenu(menu)
	tray.OnClick(s.showWindow)
}

func (s *shell) showWindow() {
	if s.window == nil {
		return
	}
	if s.window.IsMinimised() {
		s.window.Restore()
	}
	s.window.Show()
	s.window.Focus()
}

func (s *shell) quit() {
	s.mu.Lock()
	s.quitting = true
	s.mu.Unlock()
	s.app.Quit()
}

func (s *shell) isQuitting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quitting
}

// onServerExit handles a server that ended by itself. Exit code 75 is a restart request (settings changed):
// start it again and point the window at the new address. Any other exit is fatal.
func (s *shell) onServerExit(code int) {
	if s.isQuitting() {
		return
	}
	if code != desktop.RestartExitCode {
		s.fatal(fmt.Sprintf("The server stopped unexpectedly (code %d).", code))
		return
	}
	go func() {
		addr, err := s.super.Start(context.Background())
		if err != nil {
			s.fatal(err.Error())
			return
		}
		application.InvokeSync(func() { setTrustedPort(addr.Port) })
		s.trustServer(addr)
		s.window.SetURL(addr.WindowURL(s.super.Token()))
	}()
}

// trustServer makes the web view accept the server's own self-signed certificate, before the page loads.
func (s *shell) trustServer(addr desktop.Addr) {
	pem, err := addr.ServerCertificatePEM()
	if err != nil {
		s.logger.Warn("not trusting the server certificate", "error", err)
		return
	}
	application.InvokeSync(func() {
		if err := allowServerCertificate(addr.LocalHost(), pem); err != nil {
			s.logger.Warn("could not trust the server certificate", "error", err)
		}
	})
}

// fatal shows the error with the last server log lines, then quits (which stops the server).
func (s *shell) fatal(message string) {
	s.logger.Error("fatal", "message", message)
	if s.isQuitting() {
		return
	}
	s.mu.Lock()
	s.quitting = true
	s.mu.Unlock()
	text := strings.TrimSpace(message + "\n\n" + s.super.LogTail())
	application.InvokeSync(func() {
		s.app.Dialog.Error().SetTitle(appTitle).SetMessage(text).Show()
	})
	s.app.Quit()
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			return path[:i]
		}
	}
	return "."
}
