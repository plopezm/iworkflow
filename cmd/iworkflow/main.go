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
	// defaultSiteAddr is the development address; the packaged defaults move the port.
	defaultSiteAddr string = "127.0.0.1:3000"
	defaultSiteRoot string = "web/static"
	readHeaderLimit        = 10 * time.Second
	shutdownTimeout        = 10 * time.Second
	exitUsage              = 2
	logLevelVar     string = "IWF_LOG_LEVEL"
)

// version is stamped at build time (-ldflags "-X main.version=...") from electron/package.json.
var version string = "dev"

func main() {
	// Loaded first, like the Rust server: the file can set the log level and every IWF_* variable.
	dotEnvPath, dotEnvErr := loadDotEnv()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel(os.Getenv(logLevelVar))}))
	if dotEnvErr != nil {
		logger.Warn("could not read the .env file", "error", dotEnvErr)
	} else if dotEnvPath != "" {
		logger.Debug("loaded the .env file", "path", dotEnvPath)
	}
	logger.Debug("iworkflow version", "version", version)
	os.Exit(run(logger, os.Args[1:]))
}

// loadDotEnv loads the .env of the working folder or its ancestors. A missing folder is not fatal.
func loadDotEnv() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
	}
	path, err := config.LoadDotEnv(dir)
	if err != nil {
		return "", fmt.Errorf("dotenv: %w", err)
	}
	return path, nil
}

// logLevel reads the log level: "debug" turns on the request log, anything else keeps info.
func logLevel(value string) slog.Level {
	if value == "debug" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
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

	if command.Version {
		fmt.Println(version)
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

	siteOptions, addrPinned, err := siteOptionsFrom(options)
	if err != nil {
		return err
	}
	// The bind address is read once at start-up: the saved network-access choice applies after a
	// restart, and an explicit --addr (or IWORKFLOW_ADDR) always takes precedence.
	prefs, err := app.LoadPreferences(ctx, handle, logger)
	if err != nil {
		return fmt.Errorf("load preferences: %w", err)
	}
	locked := addrPinned
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
	// The modules release what they hold once serving has ended. The release is registered only after the bind has
	// succeeded: a failed bind has started nothing, and a failed start releases inside bindThenStart.
	release := func() { shutdownApplication(ctx, logger, application) }
	listener, err := bindThenStart(ctx, addr.String(), listenOn, application.Start, release)
	if err != nil {
		return err
	}
	defer release()
	logAccess(ctx, logger, info, prefs)
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
	return serveUntilStopped(ctx, logger, server, listener, restartHandle)
}

// bindFunc opens the listener of the server, startFunc starts the application.
type (
	bindFunc  func(ctx context.Context, addr string) (net.Listener, error)
	startFunc func(ctx context.Context) error
)

// listenOn opens the TCP listener of the server.
func listenOn(ctx context.Context, addr string) (net.Listener, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("open listener: %w", err)
	}
	return listener, nil
}

// bindThenStart binds the address and then starts the application. The bind comes first, so a second instance on a
// taken address fails before it reconciles or sweeps the data the live instance owns: a failed bind starts nothing and
// has nothing to stop. A failed start closes the listener and calls stop, which releases what the application began;
// after a successful start the caller owns the release.
func bindThenStart(ctx context.Context, addr string, bind bindFunc, start startFunc, stop func()) (net.Listener, error) {
	listener, err := bind(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("bind %s: %w", addr, err)
	}
	if err := start(ctx); err != nil {
		_ = listener.Close()
		stop()
		return nil, fmt.Errorf("start application: %w", err)
	}
	return listener, nil
}

// shutdownApplication releases the modules within the application's shutdown budget. The signal context is already done when
// it runs, so the deadline starts from its values without its cancellation.
func shutdownApplication(ctx context.Context, logger *slog.Logger, application *app.App) {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), application.ShutdownBudget())
	defer cancel()
	if err := application.Shutdown(stopCtx); err != nil {
		logger.Warn("cannot release the modules on shutdown", "error", err)
	}
}

// siteOptionsFrom returns the site settings and whether the address was pinned: the command line
// wins, then the environment, then the packaged defaults.
func siteOptionsFrom(options launch.Options) (site.Options, bool, error) {
	value := options.Addr
	if value == "" {
		value, _ = listen.LookupSiteAddr()
	}
	pinned := value != ""
	if !pinned {
		value = defaultSiteAddr
	}
	addr, err := netip.ParseAddrPort(value)
	if err != nil {
		return site.Options{}, false, fmt.Errorf("invalid listen address (--addr or %s): %w", listen.SiteAddrVar, err)
	}
	root := defaultSiteRoot
	if options.SiteRoot != "" {
		root = options.SiteRoot
	}
	return site.WithPackagedDefaults(site.Options{SiteRoot: root, SiteAddr: addr}, options.SiteRoot != "", pinned), pinned, nil
}

// logAccess tells the operator how other devices reach the server, and warns when it is open
// without a token.
func logAccess(ctx context.Context, logger *slog.Logger, info listen.Info, prefs app.Preferences) {
	host := "<IP>"
	if ip, ok := listen.LocalNetworkIP(ctx); ok {
		host = ip.String()
	}
	logger.Info("access token", "token", prefs.Token.Value(), "link", app.AccessLink(host, info.Addr.Port(), prefs.Token))
	if info.TokenRequired {
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
