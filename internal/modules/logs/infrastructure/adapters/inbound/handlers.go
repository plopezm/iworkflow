// Package inbound holds the HTTP adapter of the logs module: the htmx fragment of the log card.
// Handlers call the use case and render; they hold no business logic.
package inbound

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/logs/application"
	"iworkflow/internal/modules/logs/presentation/components"
	"iworkflow/internal/modules/logs/presentation/state"
)

// Handlers serves the card. It depends on the use case only.
type Handlers struct {
	get *application.GetLogs
}

// NewHandlers builds the handlers over the use case of the module.
func NewHandlers(get *application.GetLogs) *Handlers {
	return &Handlers{get: get}
}

// Mount registers the card route on r.
//
//	GET /logs/card  the content of the card, which the card reloads periodically
func (h *Handlers) Mount(r chi.Router) {
	r.Get(components.LogsPath, h.body)
}

// Card returns the whole card for the request context, as the settings section embeds it.
func (h *Handlers) Card(ctx context.Context) templ.Component {
	return components.LogsCard(h.props(ctx))
}

func (h *Handlers) body(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, components.LogsBody(h.props(r.Context())))
}

func (h *Handlers) props(ctx context.Context) components.LogsCardProps {
	tail, err := h.get.Execute(ctx)
	if err != nil {
		web.LogError(ctx, "read log", err)
	}
	return state.LogsCard(ctx, tail, err != nil)
}
