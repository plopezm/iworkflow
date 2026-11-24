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
	addr     desktop.Addr
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

	// The server starts before the GUI loop so a start failure is reported without opening a window.
	addr, err := sh.super.Start(context.Background())
	if err != nil {
		sh.super.Stop()
		return fmt.Errorf("starting the server: %w\n%s", err, sh.super.LogTail())
	}
	sh.addr = addr

	sh.app = application.New(application.Options{
		Name: appTitle,
		Icon: icon,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "dev.iworkflow.desktop",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { sh.showWindow() },
		},
		OnShutdown: sh.super.Stop,
	})
	sh.window = sh.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  appTitle,
		Width:  1400,
		Height: 900,
	})
	// The certificate hook must be in place before the first load, so the URL is set once the GUI runs.
	sh.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		application.InvokeSync(func() {
			if err := trustLocalCertificate(sh.window.NativeWindow()); err != nil {
				logger.Warn("could not trust the local certificate", "error", err)
			}
		})
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

// onServerExit handles a server that ended by itself. Restart (T4) and the error dialog come in the next slice;
// for now an unexpected exit closes the app so the prototype never shows a dead window.
func (s *shell) onServerExit(code int) {
	if s.isQuitting() {
		return
	}
	s.logger.Error("the server stopped unexpectedly", "code", code, "tail", s.super.LogTail())
	s.quit()
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			return path[:i]
		}
	}
	return "."
}
