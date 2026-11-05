// Package inbound holds the HTTP adapter of the data_home module: htmx fragments of the data folder
// card. Handlers decode the form, call the use cases and render; they hold no business logic.
package inbound

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/data_home/application"
	"iworkflow/internal/modules/data_home/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/data_home/presentation/components"
	"iworkflow/internal/modules/data_home/presentation/state"
)

// Routes of the card. The move and cancel controls post and receive the whole card back.
const (
	movePath   = "/data-home/move"
	cancelPath = "/data-home/cancel"
)

// Handlers serves the card. It depends on the use cases only.
type Handlers struct {
	getHome *application.GetDataHome
	request *application.RequestDataHomeMove
	cancel  *application.CancelDataHomeMove
}

// NewHandlers builds the handlers over the use cases of the module.
func NewHandlers(getHome *application.GetDataHome, request *application.RequestDataHomeMove, cancel *application.CancelDataHomeMove) *Handlers {
	return &Handlers{getHome: getHome, request: request, cancel: cancel}
}

// Mount registers the card routes on r.
//
//	GET  /data-home/card    the card with the current folder
//	POST /data-home/move    request the move (form: destination); the server then restarts
//	POST /data-home/cancel  withdraw the pending move
func (h *Handlers) Mount(r chi.Router) {
	r.Get(components.DataHomePath, h.card)
	r.Post(movePath, h.move)
	r.Post(cancelPath, h.cancelMove)
}

// Card returns the card for the request context, as the settings section embeds it.
func (h *Handlers) Card(ctx context.Context) templ.Component {
	return h.view(ctx, state.Notice{}, false)
}

// card renders the card. The poll of a restarting card asks with restarting=1: while the move is still
// pending the server has not restarted yet, so the card must stay in its restarting state instead of
// going back to the form.
func (h *Handlers) card(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, h.view(r.Context(), state.Notice{}, r.URL.Query().Get(components.RestartingParam) == "1"))
}

// move requests the move. A refusal is shown above the controls; an accepted move shows the restarting
// card. The response is always 200: htmx swaps only successful responses by default.
func (h *Handlers) move(w http.ResponseWriter, r *http.Request) {
	form, err := dto.DecodeMoveForm(r)
	if err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := h.request.Execute(ctx, application.MoveInput{Destination: form.Destination}); err != nil {
		web.Render(w, r, http.StatusOK, h.view(ctx, NoticeOf(err), false))
		return
	}
	web.Render(w, r, http.StatusOK, h.view(ctx, state.Notice{}, true))
}

// cancelMove withdraws the pending move and renders the card again.
func (h *Handlers) cancelMove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var notice state.Notice
	if err := h.cancel.Execute(ctx); err != nil {
		notice = NoticeOf(err)
	}
	web.Render(w, r, http.StatusOK, h.view(ctx, notice, false))
}

// view reads the folder and builds the card. A failed read replaces the card with its error. The card
// is restarting only while a move is pending: once the move was applied there is nothing to wait for.
func (h *Handlers) view(ctx context.Context, notice state.Notice, restarting bool) templ.Component {
	status, err := h.getHome.Execute(ctx)
	if err != nil {
		return components.DataHomeCard(state.HomeCard(ctx, state.HomeCardData{ReadErr: NoticeOf(err), Notice: notice}))
	}
	return components.DataHomeCard(state.HomeCard(ctx, state.HomeCardData{
		Status:     dto.FromHomeStatus(status),
		Notice:     notice,
		Restarting: restarting && status.Pending != nil,
	}))
}
