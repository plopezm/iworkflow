// Command iworkflow starts the iworkflow web server. It mirrors the start-up of the Rust version: it
// reads the command line, opens the database, reads the saved network settings, listens, opens the
// browser and serves until a signal or a restart request asks it to stop.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iworkflow/internal/app"
	"iworkflow/internal/core/config"
	"iworkflow/internal/core/db"
	"iworkflow/internal/core/gitx"
	"iworkflow/internal/core/launch"
	"iworkflow/internal/core/listen"
	"iworkflow/internal/core/restart"
	"iworkflow/internal/core/site"
)

const (
	// defaultSiteAddr is the address the Leptos configuration used; the packaged defaults move it.
	defaultSiteAddr string = "127.0.0.1:3000"
	defaultSiteRoot string = "web/static"
	siteRootVar     string = "LEPTOS_SITE_ROOT"
	readHeaderLimit        = 10 * time.Second
	shutdownTimeout        = 10 * time.Second
	exitUsage              = 2
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	os.Exit(run(logger, os.Args[1:]))
}

// run returns the process exit code.
func run(logger *slog.Logger, args []string) int {
	command, err := launch.Parse(args)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return exitUsage
	}
	if command.Help {
		fmt.Println(launch.Usage)
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	restartHandle := restart.NewHandle()
	if err := serve(ctx, logger, command.Options, restartHandle); err != nil {
		logger.Error("iworkflow stopped", "error", err)
		return 1
	}
	return afterShutdown(ctx, logger, args, restartHandle)
}

// afterShutdown does what the Rust main does once the server has stopped: a supervised process exits
// with the restart code so the supervisor starts it again, an unsupervised one starts a copy of itself.
func afterShutdown(ctx context.Context, logger *slog.Logger, args []string, restartHandle *restart.Handle) int {
	switch restart.Decide(restartHandle.IsRequested(), restart.IsSupervised()) {
	case restart.ActionHandOver:
		return restart.ExitCode
	case restart.ActionRespawn:
		if err := restart.Respawn(ctx, args); err != nil {
			logger.Error("cannot restart the server", "error", err)
			return 1
		}
		return 0
	default:
		return 0
	}
}

// serve runs the server until it is stopped. It returns when the shutdown is complete.
func serve(ctx context.Context, logger *slog.Logger, options launch.Options, restartHandle *restart.Handle) error {
	// Worktrees with long paths fail on Windows without core.longpaths; it is not fatal.
	if err := gitx.EnableLongPaths(ctx); err != nil {
		logger.Warn("cannot enable core.longpaths in git", "error", err)
	}

	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	handle, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = handle.Close() }()
	if err := db.Migrate(ctx, handle); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// TODO(tasks): the tasks startup steps are not migrated yet. When the tasks module lands, reconcile
	// the interrupted tasks, sweep the orphan worktrees and start the graph and resume loops here, and
	// stop the agents and flush the task events after the server has stopped (see the Rust main).

	siteOptions, err := siteOptionsFromEnv()
	if err != nil {
		return err
	}
	// The bind address is read once at start-up: the saved network-access choice applies after a
	// restart, and an explicit LEPTOS_SITE_ADDR always takes precedence.
	prefs, err := app.LoadPreferences(ctx, handle, logger)
	if err != nil {
		return fmt.Errorf("load preferences: %w", err)
	}
	_, locked := os.LookupEnv(listen.SiteAddrVar)
	addr := listen.ResolveListenAddr(siteOptions.SiteAddr, locked, prefs.NetworkAccess, prefs.SavedPort)
	info := listen.Info{
		Addr:          addr,
		LockedByEnv:   locked,
		TokenRequired: !addr.Addr().IsLoopback() && prefs.RequireToken,
	}

	application, err := app.New(app.Deps{
		DB:          handle,
		Config:      cfg,
		Logger:      logger,
		StaticDir:   siteOptions.SiteRoot,
		Listen:      info,
		Preferences: prefs,
		Restart:     restartHandle,
	})
	if err != nil {
		return fmt.Errorf("compose application: %w", err)
	}
	logAccess(ctx, logger, info, prefs)

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr.String())
	if err != nil {
		return fmt.Errorf("bind %s: %w", addr, err)
	}
	logger.Info("listening", "url", "http://"+addr.String())
	if options.OpenBrowser {
		url := launch.BrowserURL(addr)
		if err := launch.OpenBrowser(ctx, url); err != nil {
			logger.Warn("open the URL manually in the browser", "url", url, "error", err)
		}
	}

	server := &http.Server{
		Handler:           application.Router(),
		ReadHeaderTimeout: readHeaderLimit,
	}
	serveErr := serveUntilStopped(ctx, logger, server, listener, restartHandle)
	// The HTTP server has stopped; the modules now release what they hold (live terminal shells).
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := application.Shutdown(stopCtx); err != nil {
		logger.Warn("cannot release the modules on shutdown", "error", err)
	}
	return serveErr
}

// siteOptionsFromEnv returns the site settings: the environment wins, then the packaged defaults.
func siteOptionsFromEnv() (site.Options, error) {
	value, ok := os.LookupEnv(listen.SiteAddrVar)
	if !ok {
		value = defaultSiteAddr
	}
	addr, err := netip.ParseAddrPort(value)
	if err != nil {
		return site.Options{}, fmt.Errorf("invalid %s: %w", listen.SiteAddrVar, err)
	}
	root := defaultSiteRoot
	if value, ok := os.LookupEnv(siteRootVar); ok {
		root = value
	}
	return site.WithPackagedDefaults(site.Options{SiteRoot: root, SiteAddr: addr}), nil
}

// logAccess tells the operator how other devices reach the server, and warns when it is open
// without a token.
func logAccess(ctx context.Context, logger *slog.Logger, info listen.Info, prefs app.Preferences) {
	if info.TokenRequired {
		host := "<IP>"
		if ip, ok := listen.LocalNetworkIP(ctx); ok {
			host = ip.String()
		}
		logger.Info("network access is protected by a token", "link", app.AccessLink(host, info.Addr.Port(), prefs.Token))
		return
	}
	if !info.Addr.Addr().IsLoopback() {
		logger.Warn("the server is reachable from the network WITHOUT an access token")
	}
}

// serveUntilStopped serves until a signal, a restart request or a serve failure, then shuts down.
// Open event streams do not end on their own, so the shutdown has a deadline and then forces the close.
func serveUntilStopped(ctx context.Context, logger *slog.Logger, server *http.Server, listener net.Listener, restartHandle *restart.Handle) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case <-restartHandle.Requested():
		logger.Info("restart requested from the interface")
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("open connections after the shutdown deadline; forcing the shutdown", "timeout", shutdownTimeout, "error", err)
		_ = server.Close()
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
