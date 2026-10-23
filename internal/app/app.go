// Package app is the composition root: it groups the feature modules and builds the router.
package app

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"iworkflow/internal/core/config"
	"iworkflow/internal/core/web"
)

// App holds the composed feature modules.
type App struct {
	modules   []web.Module
	logger    *slog.Logger
	staticDir string
}

// New composes every feature module. Add each new module to the list below (and only here).
func New(_ *sql.DB, _ config.Config, logger *slog.Logger, staticDir string) *App {
	return &App{
		modules:   []web.Module{},
		logger:    logger,
		staticDir: staticDir,
	}
}

// Router builds the HTTP router with the shared middlewares and every module's routes.
func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, web.Recover(a.logger), web.Locale, web.BodyLimit(web.DefaultBodyBytes))
	r.Handle("/static/*", web.StaticHandler(a.staticDir))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, module := range a.modules {
		module.Routes(r)
	}
	return r
}
