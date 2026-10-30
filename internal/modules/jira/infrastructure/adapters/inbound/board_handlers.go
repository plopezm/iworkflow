//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/components"
	"iworkflow/internal/modules/jira/presentation/pages"
	"iworkflow/internal/modules/jira/presentation/state"
)

// BoardHandlers serves the Jira issues board and lane management endpoints.
type BoardHandlers struct {
	GetConnection    *application.GetConnection
	SyncIssues       *application.SyncIssues
	ListIssues       *application.ListIssues
	CreateLane       *application.CreateLane
	UpdateLane       *application.UpdateLane
	DeleteLane       *application.DeleteLane
	SetLaneCollapsed *application.SetLaneCollapsed
	SetIssueIgnored  *application.SetIssueIgnored
}

// NewBoardHandlers constructs BoardHandlers with all necessary use cases.
func NewBoardHandlers(
	getConnection *application.GetConnection,
	syncIssues *application.SyncIssues,
	listIssues *application.ListIssues,
	createLane *application.CreateLane,
	updateLane *application.UpdateLane,
	deleteLane *application.DeleteLane,
	setLaneCollapsed *application.SetLaneCollapsed,
	setIssueIgnored *application.SetIssueIgnored,
) *BoardHandlers {
	return &BoardHandlers{
		GetConnection:    getConnection,
		SyncIssues:       syncIssues,
		ListIssues:       listIssues,
		CreateLane:       createLane,
		UpdateLane:       updateLane,
		DeleteLane:       deleteLane,
		SetLaneCollapsed: setLaneCollapsed,
		SetIssueIgnored:  setIssueIgnored,
	}
}

// MountBoardRoutes registers the Jira board and lane endpoints.
func (h *BoardHandlers) MountBoardRoutes(r chi.Router) {
	r.Post("/jira/sync", h.Sync)
	r.Get("/jira/board", h.Board)
	r.Get("/jira/lanes/new", h.NewLane)
	r.Post("/jira/lanes", h.CreateLaneHandler)
	r.Get("/jira/lanes/{id}/edit", h.EditLane)
	r.Post("/jira/lanes/{id}", h.UpdateLaneHandler)
	r.Put("/jira/lanes/{id}", h.UpdateLaneHandler)
	r.Get("/jira/lanes/{id}/confirm-delete", h.ConfirmDeleteLane)
	r.Post("/jira/lanes/{id}/delete", h.DeleteLaneHandler)
	r.Delete("/jira/lanes/{id}", h.DeleteLaneHandler)
	r.Post("/jira/lanes/{id}/collapse", h.ToggleCollapse)
	r.Post("/jira/lanes/{id}/toggle", h.ToggleCollapse)
	r.Get("/jira/lanes/cancel", h.CancelLane)
	r.Post("/jira/issues/{key}/ignore", h.IgnoreIssue)
}

// Mount is an alias for MountBoardRoutes.
func (h *BoardHandlers) Mount(r chi.Router) {
	h.MountBoardRoutes(r)
}

type boardRenderOpts struct {
	filter          dto.IssueFilterDTO
	syncMsg         *string
	failedLanes     []string
	laneNotice      *string
	laneForm        state.LaneFormView
	confirmDeleteID string
}

func extractFilter(r *http.Request) dto.IssueFilterDTO {
	return state.BuildFilter(
		r.URL.Query().Get("search"),
		r.URL.Query().Get("status"),
		r.URL.Query().Get("local_state"),
	)
}

func (h *BoardHandlers) buildView(ctx context.Context, opts boardRenderOpts) state.IssuesPanelView {
	listing, err := h.ListIssues.Execute(ctx, opts.filter.ToDomain())
	if err != nil && opts.laneNotice == nil {
		msg := err.Error()
		opts.laneNotice = &msg
	}
	dtoListing := dto.ListingFromDomain(listing)
	lanes := make([]state.LaneView, len(dtoListing.Lanes))
	for i, l := range dtoListing.Lanes {
		lanes[i] = state.LaneView{
			Lane:             l.Lane,
			Issues:           l.Issues,
			Total:            l.Total,
			ConfirmingDelete: l.Lane.ID == opts.confirmDeleteID,
		}
	}
	return state.IssuesPanelView{
		SyncMessage: opts.syncMsg,
		FailedLanes: opts.failedLanes,
		LaneNotice:  opts.laneNotice,
		Filters:     opts.filter,
		Statuses:    dtoListing.Statuses,
		Lanes:       lanes,
		LaneForm:    opts.laneForm,
	}
}

func (h *BoardHandlers) renderBoard(w http.ResponseWriter, r *http.Request, opts boardRenderOpts) {
	view := h.buildView(r.Context(), opts)
	web.Render(w, r, http.StatusOK, components.IssuesPanel(view))
}

// BuildBoardView creates an IssuesPanelView for the given filter.
func (h *BoardHandlers) BuildBoardView(ctx context.Context, filter dto.IssueFilterDTO) state.IssuesPanelView {
	return h.buildView(ctx, boardRenderOpts{filter: filter})
}

// BoardOOB returns the board wrapped in out-of-band swap markup.
func (h *BoardHandlers) BoardOOB(ctx context.Context) templ.Component {
	view := h.BuildBoardView(ctx, dto.IssueFilterDTO{})
	return components.IssuesPanelOOB(view)
}

// Page renders the full Jira screen including the connection card and issues board.
func (h *BoardHandlers) Page(ctx context.Context) templ.Component {
	connection, err := h.GetConnection.Execute(ctx)
	if err != nil {
		return pages.JiraPage(state.ConnectionCardView{Err: err.Error()}, nil)
	}
	var storedDTO *dto.ConnectionDTO
	if connection != nil {
		d := dto.ConnectionFromDomain(*connection)
		storedDTO = &d
	}
	connView := state.ConnectionCardView{
		Stored:  storedDTO,
		Editing: storedDTO == nil,
	}
	if storedDTO == nil {
		return pages.JiraPage(connView, nil)
	}
	boardView := h.BuildBoardView(ctx, dto.IssueFilterDTO{})
	return pages.JiraPage(connView, &boardView)
}

// Sync imports the issues of every lane from Jira.
func (h *BoardHandlers) Sync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := extractFilter(r)
	outcome, err := h.SyncIssues.Execute(ctx)
	var syncMsg *string
	var failedLanes []string
	var laneNotice *string
	if err != nil {
		msg := err.Error()
		laneNotice = &msg
	} else {
		dtoOutcome := dto.SyncOutcomeFromDomain(outcome)
		loc := i18n.StateFrom(ctx).Locale
		msg := state.SyncMessage(dtoOutcome, loc)
		syncMsg = &msg
		failedLanes = outcome.FailedLanes
	}
	h.renderBoard(w, r, boardRenderOpts{
		filter:      filter,
		syncMsg:     syncMsg,
		failedLanes: failedLanes,
		laneNotice:  laneNotice,
	})
}

// Board returns the refreshed board with current filters applied.
func (h *BoardHandlers) Board(w http.ResponseWriter, r *http.Request) {
	filter := extractFilter(r)
	h.renderBoard(w, r, boardRenderOpts{
		filter: filter,
	})
}

// NewLane opens the lane creation form.
func (h *BoardHandlers) NewLane(w http.ResponseWriter, r *http.Request) {
	filter := extractFilter(r)
	h.renderBoard(w, r, boardRenderOpts{
		filter: filter,
		laneForm: state.LaneFormView{
			Mode: state.LaneFormCreating,
		},
	})
}

// CreateLaneHandler parses the form, creates the lane, syncs it, and returns the board.
func (h *BoardHandlers) CreateLaneHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	jql := strings.TrimSpace(r.FormValue("jql"))
	filter := extractFilter(r)

	if !state.CanSaveLane(name, jql) {
		h.renderBoard(w, r, boardRenderOpts{
			filter: filter,
			laneForm: state.LaneFormView{
				Mode: state.LaneFormCreating,
				Name: name,
				JQL:  jql,
				Err:  "El nombre y la consulta JQL son obligatorios.",
			},
		})
		return
	}

	saved, err := h.CreateLane.Execute(ctx, name, jql)
	if err != nil {
		h.renderBoard(w, r, boardRenderOpts{
			filter: filter,
			laneForm: state.LaneFormView{
				Mode: state.LaneFormCreating,
				Name: name,
				JQL:  jql,
				Err:  err.Error(),
			},
		})
		return
	}

	savedDTO := dto.SavedLaneFromDomain(saved)
	loc := i18n.StateFrom(ctx).Locale
	laneNotice := state.LaneSyncNotice(savedDTO, loc)

	h.renderBoard(w, r, boardRenderOpts{
		filter:     filter,
		laneNotice: laneNotice,
	})
}

// EditLane opens the lane editing form.
func (h *BoardHandlers) EditLane(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	filter := extractFilter(r)

	listing, err := h.ListIssues.Execute(ctx, models.IssueFilter{})
	if err != nil {
		msg := err.Error()
		h.renderBoard(w, r, boardRenderOpts{filter: filter, laneNotice: &msg})
		return
	}
	var target *models.Lane
	for _, l := range listing.Lanes {
		if l.Lane.ID == id {
			target = &l.Lane
			break
		}
	}
	if target == nil {
		msg := "Carril no encontrado"
		h.renderBoard(w, r, boardRenderOpts{filter: filter, laneNotice: &msg})
		return
	}

	h.renderBoard(w, r, boardRenderOpts{
		filter: filter,
		laneForm: state.LaneFormView{
			Mode:   state.LaneFormEditing,
			LaneID: target.ID,
			Name:   target.Name,
			JQL:    target.JQL,
		},
	})
}

// UpdateLaneHandler parses the form, updates the lane, and returns the board.
func (h *BoardHandlers) UpdateLaneHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	jql := strings.TrimSpace(r.FormValue("jql"))
	filter := extractFilter(r)

	if !state.CanSaveLane(name, jql) {
		h.renderBoard(w, r, boardRenderOpts{
			filter: filter,
			laneForm: state.LaneFormView{
				Mode:   state.LaneFormEditing,
				LaneID: id,
				Name:   name,
				JQL:    jql,
				Err:    "El nombre y la consulta JQL son obligatorios.",
			},
		})
		return
	}

	saved, err := h.UpdateLane.Execute(ctx, id, name, jql)
	if err != nil {
		h.renderBoard(w, r, boardRenderOpts{
			filter: filter,
			laneForm: state.LaneFormView{
				Mode:   state.LaneFormEditing,
				LaneID: id,
				Name:   name,
				JQL:    jql,
				Err:    err.Error(),
			},
		})
		return
	}

	savedDTO := dto.SavedLaneFromDomain(saved)
	loc := i18n.StateFrom(ctx).Locale
	laneNotice := state.LaneSyncNotice(savedDTO, loc)

	h.renderBoard(w, r, boardRenderOpts{
		filter:     filter,
		laneNotice: laneNotice,
	})
}

// ConfirmDeleteLane shows the delete confirmation panel on the specified lane.
func (h *BoardHandlers) ConfirmDeleteLane(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	filter := extractFilter(r)
	h.renderBoard(w, r, boardRenderOpts{
		filter:          filter,
		confirmDeleteID: id,
	})
}

// DeleteLaneHandler deletes a custom lane.
func (h *BoardHandlers) DeleteLaneHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	filter := extractFilter(r)

	if err := h.DeleteLane.Execute(ctx, id); err != nil {
		msg := err.Error()
		h.renderBoard(w, r, boardRenderOpts{
			filter:     filter,
			laneNotice: &msg,
		})
		return
	}

	h.renderBoard(w, r, boardRenderOpts{
		filter: filter,
	})
}

// ToggleCollapse toggles the collapsed state of a lane.
func (h *BoardHandlers) ToggleCollapse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	filter := extractFilter(r)

	listing, err := h.ListIssues.Execute(ctx, models.IssueFilter{})
	if err != nil {
		msg := err.Error()
		h.renderBoard(w, r, boardRenderOpts{filter: filter, laneNotice: &msg})
		return
	}
	var target *models.Lane
	for _, l := range listing.Lanes {
		if l.Lane.ID == id {
			target = &l.Lane
			break
		}
	}
	if target == nil {
		msg := "Carril no encontrado"
		h.renderBoard(w, r, boardRenderOpts{filter: filter, laneNotice: &msg})
		return
	}

	newCollapsed := !target.Collapsed
	if c := r.URL.Query().Get("collapsed"); c != "" {
		newCollapsed = c == "true" || c == "1"
	}

	if err := h.SetLaneCollapsed.Execute(ctx, id, newCollapsed); err != nil {
		msg := err.Error()
		h.renderBoard(w, r, boardRenderOpts{filter: filter, laneNotice: &msg})
		return
	}

	h.renderBoard(w, r, boardRenderOpts{
		filter: filter,
	})
}

// CancelLane resets any open lane form or confirm delete notice.
func (h *BoardHandlers) CancelLane(w http.ResponseWriter, r *http.Request) {
	filter := extractFilter(r)
	h.renderBoard(w, r, boardRenderOpts{
		filter: filter,
	})
}

// IgnoreIssue toggles the local ignored status of an issue.
func (h *BoardHandlers) IgnoreIssue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")
	filter := extractFilter(r)

	ignoredParam := r.URL.Query().Get("ignored")
	if ignoredParam == "" {
		ignoredParam = r.FormValue("ignored")
	}
	ignored := ignoredParam == "true" || ignoredParam == "1"

	if err := h.SetIssueIgnored.Execute(ctx, key, ignored); err != nil {
		msg := err.Error()
		h.renderBoard(w, r, boardRenderOpts{filter: filter, laneNotice: &msg})
		return
	}

	h.renderBoard(w, r, boardRenderOpts{
		filter: filter,
	})
}
