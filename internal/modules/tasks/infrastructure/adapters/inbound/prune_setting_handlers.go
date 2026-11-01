//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/presentation/components"
)

// PruneSettingHandlers serves the card with the checkbox that switches the automatic deletion of old merged tasks.
type PruneSettingHandlers struct {
	preference outbound.MergedPrunePreference
}

// NewPruneSettingHandlers builds the handlers on the stored switch.
func NewPruneSettingHandlers(preference outbound.MergedPrunePreference) *PruneSettingHandlers {
	return &PruneSettingHandlers{preference: preference}
}

// Mount registers the routes: GET shows the card, POST saves the checkbox (form field enabled, present when on).
func (h *PruneSettingHandlers) Mount(r chi.Router) {
	r.Get(components.PruneSettingPath, h.show)
	r.Post(components.PruneSettingPath, h.save)
}

// Card is the card for the context: the stored switch, or the read error.
func (h *PruneSettingHandlers) Card(ctx context.Context) templ.Component {
	return h.card(ctx, "")
}

func (h *PruneSettingHandlers) card(ctx context.Context, saveErr string) templ.Component {
	enabled, err := h.preference.Enabled(ctx)
	if err != nil {
		return components.PruneSettingCard(components.NewPruneSettingProps(ctx, true, err.Error()))
	}
	return components.PruneSettingCard(components.NewPruneSettingProps(ctx, enabled, saveErr))
}

func (h *PruneSettingHandlers) show(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, h.Card(r.Context()))
}

// save stores the checkbox and answers with the card read again, so a failed save shows the stored value.
func (h *PruneSettingHandlers) save(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	saveErr := ""
	if err := h.preference.SetEnabled(r.Context(), r.PostForm.Get("enabled") == "true"); err != nil {
		saveErr = err.Error()
	}
	web.Render(w, r, http.StatusOK, h.card(r.Context(), saveErr))
}
