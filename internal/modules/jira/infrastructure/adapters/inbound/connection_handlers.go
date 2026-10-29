//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/components"
	"iworkflow/internal/modules/jira/presentation/pages"
	"iworkflow/internal/modules/jira/presentation/state"
)

// ConnectionHandlers serves the Jira connection card endpoints.
type ConnectionHandlers struct {
	GetConnection    *application.GetConnection
	SaveConnection   *application.SaveConnection
	DeleteConnection *application.DeleteConnection
	boardHandlers    *BoardHandlers
}

// SetBoardHandlers sets the board handlers reference for integrated page and card rendering.
func (h *ConnectionHandlers) SetBoardHandlers(b *BoardHandlers) {
	h.boardHandlers = b
}

// NewConnectionHandlers constructs the connection handlers with their required use cases.
func NewConnectionHandlers(
	getConnection *application.GetConnection,
	saveConnection *application.SaveConnection,
	deleteConnection *application.DeleteConnection,
) *ConnectionHandlers {
	return &ConnectionHandlers{
		GetConnection:    getConnection,
		SaveConnection:   saveConnection,
		DeleteConnection: deleteConnection,
	}
}

// MountConnectionRoutes registers the Jira connection routes on the given router.
func (h *ConnectionHandlers) MountConnectionRoutes(r chi.Router) {
	r.Get("/jira/connection", h.Get)
	r.Post("/jira/connection", h.Save)
	r.Get("/jira/connection/edit", h.Edit)
	r.Get("/jira/connection/cancel", h.Cancel)
	r.Post("/jira/connection/disconnect", h.Disconnect)
	r.Delete("/jira/connection", h.Disconnect)
}

// Mount registers the routes; alias for MountConnectionRoutes.
func (h *ConnectionHandlers) Mount(r chi.Router) {
	h.MountConnectionRoutes(r)
}

// MountConnectionRoutes is a package-level helper to mount connection routes.
func MountConnectionRoutes(r chi.Router, h *ConnectionHandlers) {
	h.MountConnectionRoutes(r)
}

// SettingsContent returns a function that renders the JiraConnectionCard for the settings screen.
func (h *ConnectionHandlers) SettingsContent() func(context.Context) templ.Component {
	return func(ctx context.Context) templ.Component {
		connection, err := h.GetConnection.Execute(ctx)
		if err != nil {
			return components.JiraConnectionCard(state.ConnectionCardView{Err: err.Error()})
		}
		var storedDTO *dto.ConnectionDTO
		if connection != nil {
			d := dto.ConnectionFromDomain(*connection)
			storedDTO = &d
		}
		view := state.ConnectionCardView{
			Stored:  storedDTO,
			Editing: storedDTO == nil,
		}
		return components.JiraConnectionCard(view)
	}
}

// Page renders the full Jira screen for the dock tab.
func (h *ConnectionHandlers) Page(ctx context.Context) templ.Component {
	if h.boardHandlers != nil {
		return h.boardHandlers.Page(ctx)
	}
	connection, err := h.GetConnection.Execute(ctx)
	if err != nil {
		return pages.JiraPage(state.ConnectionCardView{Err: err.Error()}, nil)
	}
	var storedDTO *dto.ConnectionDTO
	if connection != nil {
		d := dto.ConnectionFromDomain(*connection)
		storedDTO = &d
	}
	view := state.ConnectionCardView{
		Stored:  storedDTO,
		Editing: storedDTO == nil,
	}
	return pages.JiraPage(view, nil)
}

// Get renders the ConnectionCard: summary if connected and not editing, or form if not connected.
func (h *ConnectionHandlers) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	connection, err := h.GetConnection.Execute(ctx)
	if err != nil {
		view := state.ConnectionCardView{
			Err: err.Error(),
		}
		web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
		return
	}

	var storedDTO *dto.ConnectionDTO
	if connection != nil {
		d := dto.ConnectionFromDomain(*connection)
		storedDTO = &d
	}

	confirmingDelete := r.URL.Query().Get("confirm_delete") == "1" || r.URL.Query().Get("confirm") == "1"
	view := state.ConnectionCardView{
		Stored:           storedDTO,
		Editing:          storedDTO == nil,
		ConfirmingDelete: confirmingDelete,
	}
	web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
}

// Save parses the form, executes SaveConnection, and renders the card.
// On success, renders the connected summary. On error, renders the form with the error and prefilled URL.
func (h *ConnectionHandlers) Save(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	baseURL := strings.TrimSpace(r.FormValue("base_url"))
	token := r.FormValue("token")
	acceptCertsVal := r.FormValue("accept_invalid_certs")
	acceptInvalidCerts := acceptCertsVal == "true" || acceptCertsVal == "on" || acceptCertsVal == "1"

	existing, _ := h.GetConnection.Execute(ctx)
	var storedDTO *dto.ConnectionDTO
	if existing != nil {
		d := dto.ConnectionFromDomain(*existing)
		storedDTO = &d
	}

	input := application.SaveConnectionInput{
		BaseURL:            baseURL,
		Token:              token,
		AcceptInvalidCerts: acceptInvalidCerts,
	}

	saved, err := h.SaveConnection.Execute(ctx, input)
	if err != nil {
		view := state.ConnectionCardView{
			Editing:                true,
			Stored:                 storedDTO,
			FormURL:                baseURL,
			FormAcceptInvalidCerts: acceptInvalidCerts,
			Err:                    err.Error(),
		}
		web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
		return
	}

	savedDTO := dto.ConnectionFromDomain(saved)
	view := state.ConnectionCardView{
		Editing: false,
		Stored:  &savedDTO,
	}
	web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
	if h.boardHandlers != nil {
		_ = h.boardHandlers.BoardOOB(ctx).Render(ctx, w)
	}
}

// Edit renders the form prefilled with the stored URL and settings.
func (h *ConnectionHandlers) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	connection, err := h.GetConnection.Execute(ctx)
	if err != nil {
		view := state.ConnectionCardView{
			Editing: true,
			Err:     err.Error(),
		}
		web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
		return
	}

	var storedDTO *dto.ConnectionDTO
	var formURL string
	var acceptCerts bool
	if connection != nil {
		d := dto.ConnectionFromDomain(*connection)
		storedDTO = &d
		formURL = connection.BaseURL()
		acceptCerts = connection.AcceptInvalidCerts()
	}

	view := state.ConnectionCardView{
		Editing:                true,
		Stored:                 storedDTO,
		FormURL:                formURL,
		FormAcceptInvalidCerts: acceptCerts,
	}
	web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
}

// Cancel renders the summary if connected, or empty form if not connected.
func (h *ConnectionHandlers) Cancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	connection, err := h.GetConnection.Execute(ctx)
	if err != nil {
		view := state.ConnectionCardView{
			Err: err.Error(),
		}
		web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
		return
	}

	var storedDTO *dto.ConnectionDTO
	if connection != nil {
		d := dto.ConnectionFromDomain(*connection)
		storedDTO = &d
	}

	view := state.ConnectionCardView{
		Stored:           storedDTO,
		Editing:          storedDTO == nil,
		ConfirmingDelete: false,
	}
	web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
}

// Disconnect executes DeleteConnection and renders the empty connection form.
func (h *ConnectionHandlers) Disconnect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.DeleteConnection.Execute(ctx); err != nil {
		var storedDTO *dto.ConnectionDTO
		if connection, getErr := h.GetConnection.Execute(ctx); getErr == nil && connection != nil {
			d := dto.ConnectionFromDomain(*connection)
			storedDTO = &d
		}
		view := state.ConnectionCardView{
			Stored: storedDTO,
			Err:    err.Error(),
		}
		web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
		return
	}

	view := state.ConnectionCardView{
		Stored:  nil,
		Editing: true,
	}
	web.Render(w, r, http.StatusOK, components.JiraConnectionCard(view))
	if h.boardHandlers != nil {
		_, _ = w.Write([]byte(`<div id="jira-board" hx-swap-oob="true"></div>`))
	}
}
