// Package inbound holds the HTTP adapters of the usage_limits module: the htmx fragment of the usage bar.
// Handlers call the use case and render; they hold no business logic.
package inbound

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/usage_limits/application"
	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/usage_limits/presentation/components"
	"iworkflow/internal/modules/usage_limits/presentation/state"
)

// Handlers serves the usage bar. It depends on the use case and the clock only.
type Handlers struct {
	get   *application.GetUsage
	clock outbound.ClockPort
}

// NewHandlers builds the handlers over the use case.
func NewHandlers(get *application.GetUsage, clock outbound.ClockPort) *Handlers {
	return &Handlers{get: get, clock: clock}
}

// Mount registers the usage bar fragment on r.
//
//	GET /usage-limits/bar   the usage bar (loaded on page load, then every 60 seconds)
func (h *Handlers) Mount(r chi.Router) {
	r.Get(components.BarPath, h.bar)
}

// bar renders the usage bar. The use case only fails when the request was cancelled; then nothing is written,
// because nobody is left to read the response.
func (h *Handlers) bar(w http.ResponseWriter, r *http.Request) {
	report, err := h.get.Execute(r.Context())
	if err != nil {
		return
	}
	web.Render(w, r, http.StatusOK, components.UsageBar(state.Bar(dto.FromReport(report), h.clock.Now())))
}
