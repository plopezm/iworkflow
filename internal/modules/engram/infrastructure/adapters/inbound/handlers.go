// Package inbound serves the Engram server settings card.
//
//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/presentation/components"
	"iworkflow/internal/modules/engram/presentation/state"
)

// UseCases are the parts of the use cases the card needs.
type UseCases struct {
	Get   getSettings
	Save  saveSettings
	Apply applySettings
	Check checkServer
}

type getSettings interface {
	Execute(ctx context.Context) (models.ServerSettings, error)
}

type saveSettings interface {
	Execute(ctx context.Context, raw string) error
}

type applySettings interface {
	Execute(ctx context.Context) (models.ServerSettings, models.StartOutcome, error)
}

type checkServer interface {
	Execute(ctx context.Context, baseURL string) bool
}

// Handlers serves the card. It holds no business rule: it reads the form, calls a use case and renders.
type Handlers struct {
	uc     UseCases
	logger *slog.Logger
}

// NewHandlers builds the handlers.
func NewHandlers(uc UseCases, logger *slog.Logger) *Handlers {
	return &Handlers{uc: uc, logger: logger}
}

// Mount registers the card routes.
func (h *Handlers) Mount(r chi.Router) {
	r.Route("/engram/settings", func(r chi.Router) {
		r.Get("/", h.show)
		r.Post("/", h.save)
		r.Post("/reset", h.reset)
	})
}

func (h *Handlers) show(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, state.StatusNone, nil)
}

func (h *Handlers) save(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, r, state.StatusNone, err)
		return
	}
	h.saveAndApply(w, r, r.PostForm.Get("url"))
}

func (h *Handlers) reset(w http.ResponseWriter, r *http.Request) {
	h.saveAndApply(w, r, "")
}

// saveAndApply stores the address and applies it: the client switches to it and Engram is started if it is off.
func (h *Handlers) saveAndApply(w http.ResponseWriter, r *http.Request, raw string) {
	if err := h.uc.Save.Execute(r.Context(), raw); err != nil {
		h.render(w, r, state.StatusNone, err)
		return
	}
	_, outcome, err := h.uc.Apply.Execute(r.Context())
	switch {
	case errors.Is(err, engramerrors.ErrStartFailed), err != nil && outcome == "":
		h.logger.Warn("engram could not be started", "error", err)
		h.render(w, r, state.StatusStartFailed, nil)
	default:
		h.render(w, r, statusOf(outcome), nil)
	}
}

func statusOf(outcome models.StartOutcome) state.Status {
	switch outcome {
	case models.OutcomeRunning:
		return state.StatusRunning
	case models.OutcomeStarted:
		return state.StatusStarted
	case models.OutcomeSkipped:
		return state.StatusSkipped
	default:
		return state.StatusNone
	}
}

// render answers with the whole card. actionErr is a failed action, if any.
func (h *Handlers) render(w http.ResponseWriter, r *http.Request, status state.Status, actionErr error) {
	settings, err := h.uc.Get.Execute(r.Context())
	if err != nil {
		h.logger.Error("read the engram settings", "error", err)
		web.Render(w, r, http.StatusOK, components.ServerCard(state.SettingsView{DefaultURL: models.DefaultServerURL}.
			WithError(components.SaveFailedText(r.Context()))))
		return
	}
	view := state.SettingsView{
		URL:        settings.URL,
		Source:     string(settings.Source),
		DefaultURL: models.DefaultServerURL,
		Reachable:  h.uc.Check.Execute(r.Context(), settings.URL),
		AutoStart:  settings.AutoStart(),
		Status:     status,
	}
	if actionErr != nil {
		view = view.WithError(h.actionMessage(r.Context(), actionErr))
	}
	web.Render(w, r, http.StatusOK, components.ServerCard(view))
}

func (h *Handlers) actionMessage(ctx context.Context, err error) string {
	if errors.Is(err, engramerrors.ErrInvalidServerURL) {
		return components.InvalidURLText(ctx)
	}
	h.logger.Error("save the engram address", "error", err)
	return components.SaveFailedText(ctx)
}
