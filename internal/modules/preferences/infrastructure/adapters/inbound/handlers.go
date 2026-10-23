package inbound

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/preferences/application"
	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/preferences/presentation/components"
	"iworkflow/internal/modules/preferences/presentation/state"
)

// UseCases are the application use cases the handlers call. The composition root builds them.
type UseCases struct {
	SaveLanguage           *application.SaveLanguage
	BuildNetworkAccess     *application.BuildNetworkAccess
	SaveNetworkAccess      *application.SaveNetworkAccess
	SaveListenPort         *application.SaveListenPort
	SaveRequireAccessToken *application.SaveRequireAccessToken
	RegenerateAccessToken  *application.RegenerateAccessToken
	RequestServerRestart   *application.RequestServerRestart
}

// Handlers answers the settings actions. Each action runs one use case, then renders the network
// card from fresh server state, so the card shows what was actually stored.
//
// Business errors (an invalid port, a storage failure) are shown inside the card with HTTP 200:
// htmx only swaps successful responses by default, and the dock does not change that. Malformed
// requests get 400.
type Handlers struct {
	logger   *slog.Logger
	guard    *AccessGuard
	useCases UseCases
}

// NewHandlers builds the handlers. guard is the live access guard, updated when the token changes.
func NewHandlers(logger *slog.Logger, guard *AccessGuard, useCases UseCases) *Handlers {
	return &Handlers{logger: logger, guard: guard, useCases: useCases}
}

// Routes mounts the settings endpoints under /preferences.
func (h *Handlers) Routes(r chi.Router) {
	r.Route("/preferences", func(r chi.Router) {
		r.Get("/network", h.showNetwork)
		r.Get("/network/target", h.showTarget)
		r.Post("/network", h.saveNetwork)
		r.Post("/port", h.savePort)
		r.Post("/require-token", h.saveRequireToken)
		r.Post("/token/regenerate", h.regenerateToken)
		r.Post("/restart", h.restart)
		r.Post("/language/{code}", h.saveLanguage)
	})
}

func (h *Handlers) showNetwork(w http.ResponseWriter, r *http.Request) {
	h.renderNetwork(w, r, nil, false)
}

// showTarget renders the link and QR code of the interface picked in the selector.
func (h *Handlers) showTarget(w http.ResponseWriter, r *http.Request) {
	view, err := h.networkView(r.Context())
	if err != nil {
		h.logger.Warn("could not read the network access state", "error", err)
		web.Render(w, r, http.StatusOK, components.NetworkTarget(nil, false))
		return
	}
	target, ok := view.SelectedTarget(r.URL.Query().Get("address"))
	if !ok {
		web.Render(w, r, http.StatusOK, components.NetworkTarget(nil, false))
		return
	}
	web.Render(w, r, http.StatusOK, components.NetworkTarget(&target, view.Access.RequireToken))
}

func (h *Handlers) saveNetwork(w http.ResponseWriter, r *http.Request) {
	enabled, err := dto.ParseBoolField(r, "enabled")
	if err != nil {
		badRequest(w, err)
		return
	}
	_, saveErr := h.useCases.SaveNetworkAccess.Execute(r.Context(), enabled)
	h.renderNetwork(w, r, saveErr, false)
}

func (h *Handlers) savePort(w http.ResponseWriter, r *http.Request) {
	port, err := dto.ParseListenPortField(r, "port")
	if err != nil {
		var invalid *domainerrors.InvalidPortError
		if !errors.As(err, &invalid) {
			badRequest(w, err)
			return
		}
		h.renderNetwork(w, r, err, false)
		return
	}
	_, saveErr := h.useCases.SaveListenPort.Execute(r.Context(), port.Port())
	h.renderNetwork(w, r, saveErr, false)
}

func (h *Handlers) saveRequireToken(w http.ResponseWriter, r *http.Request) {
	required, err := dto.ParseBoolField(r, "required")
	if err != nil {
		badRequest(w, err)
		return
	}
	_, saveErr := h.useCases.SaveRequireAccessToken.Execute(r.Context(), required)
	h.renderNetwork(w, r, saveErr, false)
}

// regenerateToken replaces the token. The old one stops working at once; this browser gets the new
// one in its cookie so it stays signed in, as the Rust build did.
func (h *Handlers) regenerateToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.useCases.RegenerateAccessToken.Execute(r.Context())
	if err == nil {
		h.guard.SetToken(token)
		if h.guard.IsEnforcing() {
			w.Header().Add("Set-Cookie", h.guard.SessionCookie())
		}
	}
	h.renderNetwork(w, r, err, false)
}

// restart asks the process to restart. The card shows the restarting state and polls until the
// server answers again.
func (h *Handlers) restart(w http.ResponseWriter, r *http.Request) {
	h.useCases.RequestServerRestart.Execute()
	h.renderNetwork(w, r, nil, true)
}

// saveLanguage stores the language and makes this browser use it: the cookie is set and the page
// reloads (HX-Refresh). A storage failure is logged and the choice still applies to this browser,
// so the click is never lost.
func (h *Handlers) saveLanguage(w http.ResponseWriter, r *http.Request) {
	language, err := models.ParseUILanguage(chi.URLParam(r, "code"))
	if err != nil {
		badRequest(w, err)
		return
	}
	if _, saveErr := h.useCases.SaveLanguage.Execute(r.Context(), language.Code()); saveErr != nil {
		h.logger.Warn("could not save the chosen language", "error", saveErr)
	}
	w.Header().Add("Set-Cookie", languageSetCookie(language.Code()))
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}

// renderNetwork answers with the whole card. actionErr is the failed action, if any; restarting
// marks a restart that was accepted.
func (h *Handlers) renderNetwork(w http.ResponseWriter, r *http.Request, actionErr error, restarting bool) {
	view, err := h.networkView(r.Context())
	if err != nil {
		h.logger.Warn("could not read the network access state", "error", err)
		web.Render(w, r, http.StatusOK, components.NetworkAccessError(err.Error()))
		return
	}
	if actionErr != nil {
		view = view.WithError(actionErr.Error())
	}
	if restarting {
		view = view.WithRestarting()
	}
	web.Render(w, r, http.StatusOK, components.NetworkAccessCard(view))
}

// networkView reads the card state and maps it to the view-model.
func (h *Handlers) networkView(ctx context.Context) (state.NetworkAccessView, error) {
	status, err := h.useCases.BuildNetworkAccess.Execute(ctx)
	if err != nil {
		return state.NetworkAccessView{}, err //nolint:wrapcheck // storage errors reach the card unchanged
	}
	return state.NewNetworkAccessView(toNetworkAccessDTO(status)), nil
}

func badRequest(w http.ResponseWriter, err error) {
	http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
}
