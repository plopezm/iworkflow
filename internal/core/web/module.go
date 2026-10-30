// Package web holds the HTTP plumbing shared by every feature: the module contract,
// middlewares, static assets and htmx/SSE helpers.
package web

import "github.com/go-chi/chi/v5"

// Module is implemented by every feature's composition root (`infrastructure/di`).
// It mounts the feature's routes; the application composes modules in internal/app.
type Module interface {
	Routes(r chi.Router)
}
