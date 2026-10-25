//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Dock route names of the board. The shell opens them by path; a module link can reuse them by name.
const (
	BoardRouteName          = "board"
	BoardWorkspaceRouteName = "board_workspace"
)

// boardRefreshEvent is the htmx event the list refreshes on after a create (see BoardList).
const boardRefreshEvent = "board:refresh"

// BoardUseCases are what the board reads and creates. The workspaces come through the outbound lookup,
// so the presentation never reaches another module.
type BoardUseCases struct {
	List       *application.ListTasks
	ListDone   *application.ListDoneTasks
	Create     *application.CreateTask
	Workspaces outbound.WorkspaceLookup
}

// BoardHandlers serves the board tabs and their htmx requests. Every htmx response is a 200: htmx 2 does
// not swap error statuses, so a domain error is rendered as the fragment itself.
type BoardHandlers struct {
	uc  BoardUseCases
	now func() time.Time
}

// NewBoardHandlers builds the handlers over the use cases. now is the clock the stages are read at.
func NewBoardHandlers(uc BoardUseCases, now func() time.Time) *BoardHandlers {
	return &BoardHandlers{uc: uc, now: now}
}

// Tabs returns the dock routes of the board: the home address and the board of one workspace.
func (h *BoardHandlers) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    BoardRouteName,
			Pattern: "/",
			Icon:    "▦",
			Title: func(ctx context.Context, _ dock.Params) string {
				return h.title(ctx, "")
			},
			Render: func(ctx context.Context, _ dock.Params) templ.Component {
				return h.boardTab(ctx, "")
			},
		},
		{
			Name:    BoardWorkspaceRouteName,
			Pattern: "/board/{id}",
			Icon:    "▦",
			Title: func(ctx context.Context, params dock.Params) string {
				return h.title(ctx, params["id"])
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.boardTab(ctx, params["id"])
			},
		},
	}
}

// Mount registers the htmx endpoints of the board: the list, the create dialog and the create itself.
func (h *BoardHandlers) Mount(r chi.Router) {
	r.Get("/board/list", h.list)
	r.Get("/board/sidebar", h.sidebar)
	r.Get("/board/tasks/new", h.newTask)
	r.Get("/board/tasks/new/close", h.closeNewTask)
	r.Post("/board/tasks", h.createTask)
}

// title is the tab title of the board of the requested workspace; an unknown one keeps the plain name.
func (h *BoardHandlers) title(ctx context.Context, requested string) string {
	choice := state.ChooseWorkspace(requested, h.workspaceOptions(ctx))
	if choice.Kind != state.ChoiceSelected {
		return components.BoardTitle(ctx, "")
	}
	return components.BoardTitle(ctx, choice.Name)
}

// boardTab renders the board of the requested workspace with the completed tasks hidden, as the Rust board does.
func (h *BoardHandlers) boardTab(ctx context.Context, requested string) templ.Component {
	return pages.BoardPage(h.view(ctx, requested, false, 1))
}

// list serves the list fragment for the address in the query: the workspace, the completed toggle and the
// number of loaded pages. It is what the controls, the load-more button and the task stream request.
func (h *BoardHandlers) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	showCompleted := query.Get("completed") == "1"
	donePages := state.ClampPages(parseInt(query.Get("pages"), 1))
	view := h.view(r.Context(), query.Get("workspace"), showCompleted, donePages)
	web.Render(w, r, http.StatusOK, components.BoardList(view))
}

// sidebar serves the board part of the sidebar, which refreshes itself (see components.BoardSidebar).
func (h *BoardHandlers) sidebar(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, h.Sidebar(r.Context()))
}

// newTask opens the create dialog, with the workspace the board shows selected.
func (h *BoardHandlers) newTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	form := state.CreateForm{
		Workspaces:  h.workspaceOptions(ctx),
		WorkspaceID: r.URL.Query().Get("workspace"),
		Priority:    state.DefaultPriority,
	}
	web.Render(w, r, http.StatusOK, components.CreateTaskDialog(form))
}

// closeNewTask closes the create dialog without creating anything.
func (h *BoardHandlers) closeNewTask(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, components.CreateTaskResult(""))
}

// createTask creates the task from the dialog. A domain error keeps the dialog open with the typed values.
// A created task closes the dialog, refreshes the list and, when its start failed, shows the notice.
func (h *BoardHandlers) createTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		web.Render(w, r, http.StatusOK, components.CreateTaskDialog(h.createForm(ctx, dto.CreateTaskDTO{}, err.Error())))
		return
	}
	input := dto.CreateTaskFromForm(r.PostForm)
	newTask, err := input.ToNewTask()
	if err != nil {
		web.Render(w, r, http.StatusOK, components.CreateTaskDialog(h.createForm(ctx, input, err.Error())))
		return
	}
	created, err := h.uc.Create.Execute(ctx, newTask)
	if err != nil {
		web.Render(w, r, http.StatusOK, components.CreateTaskDialog(h.createForm(ctx, input, err.Error())))
		return
	}
	startError := ""
	if created.StartError != nil {
		startError = created.StartError.Error()
	}
	web.Trigger(w, boardRefreshEvent)
	web.Render(w, r, http.StatusOK, components.CreateTaskResult(startError))
}

// createForm rebuilds the dialog state from what was typed, with the error to show.
func (h *BoardHandlers) createForm(ctx context.Context, input dto.CreateTaskDTO, message string) state.CreateForm {
	return state.CreateForm{
		Workspaces:  h.workspaceOptions(ctx),
		WorkspaceID: input.WorkspaceID,
		Title:       input.Title,
		Prompt:      input.Prompt,
		Priority:    priorityOrDefault(input.Priority),
		StartNow:    input.StartNow,
		Error:       message,
	}
}

// view builds what the list shows for the requested workspace. A failed read is reported in LoadError and
// leaves the other data empty, so the page still renders.
func (h *BoardHandlers) view(ctx context.Context, requested string, showCompleted bool, donePages int) state.BoardView {
	workspaces := h.workspaceOptions(ctx)
	view := state.BoardView{
		Choice:        state.ChooseWorkspace(requested, workspaces),
		Workspaces:    workspaces,
		ShowCompleted: showCompleted,
		Pages:         donePages,
	}
	if view.Choice.Kind != state.ChoiceSelected {
		return view
	}
	tasks, err := h.loadTasks(ctx, view.Choice.ID, showCompleted, donePages)
	if err != nil {
		view.LoadError = err.Error()
	}
	view.Total = len(tasks.Tasks)
	view.HasMore = showCompleted && state.DoneHasMore(tasks.LastPageLen)
	view.Groups = state.GroupByStage(tasks.Tasks)
	return view
}

// boardTasks is what loadTasks returns: the tasks of the board and the length of the last completed page
// (-1 when no completed page was requested).
type boardTasks struct {
	Tasks       []dto.TaskDTO
	LastPageLen int
}

// loadTasks reads the active tasks and, when the toggle is on, the requested pages of completed ones.
func (h *BoardHandlers) loadTasks(ctx context.Context, workspaceID string, showCompleted bool, donePages int) (boardTasks, error) {
	now := h.now()
	active, err := h.uc.List.Execute(ctx, &workspaceID)
	if err != nil {
		return boardTasks{}, fmt.Errorf("list active tasks: %w", err)
	}
	done := state.DonePages{LastPageLen: -1}
	if showCompleted {
		done, err = state.CollectDonePages(ctx, donePages, func(ctx context.Context, page int) ([]dto.TaskDTO, error) {
			rows, pageErr := h.uc.ListDone.Execute(ctx, &workspaceID, state.DonePageSize, state.DoneOffset(page))
			return tasksToDTO(rows, now), pageErr //nolint:wrapcheck // the domain error is user-visible and must stay identical
		})
		if err != nil {
			return boardTasks{}, fmt.Errorf("list completed tasks: %w", err)
		}
	}
	combined := state.CombineTasks(tasksToDTO(active, now), done.Tasks)
	return boardTasks{Tasks: state.FilterWorkspace(combined, workspaceID), LastPageLen: done.LastPageLen}, nil
}

// workspaceOptions lists the workspaces the board can show, as the switcher needs them. A failed read yields
// no options; the page then shows the empty state.
func (h *BoardHandlers) workspaceOptions(ctx context.Context) []dto.WorkspaceOptionDTO {
	refs, err := h.uc.Workspaces.List(ctx)
	if err != nil {
		return nil
	}
	options := make([]dto.WorkspaceOptionDTO, 0, len(refs))
	for _, ref := range refs {
		options = append(options, dto.WorkspaceOptionDTO{ID: ref.ID, Name: ref.Name, BaseBranch: ref.BaseBranch})
	}
	return options
}

// tasksToDTO maps the domain tasks to the board view as of now.
func tasksToDTO(tasks []models.Task, now time.Time) []dto.TaskDTO {
	mapped := make([]dto.TaskDTO, 0, len(tasks))
	for _, task := range tasks {
		mapped = append(mapped, dto.TaskFromDomain(task, now))
	}
	return mapped
}

// priorityOrDefault keeps the priority the form sent, or the default one when it sent none.
func priorityOrDefault(priority string) string {
	if priority == "" {
		return state.DefaultPriority
	}
	return priority
}

// parseInt reads a number from the query, or returns fallback when it is not one.
func parseInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// Sidebar builds the board part of the sidebar: the workspaces and the tasks that need the user. A failed
// task read leaves the attention list empty, so the sidebar still renders.
func (h *BoardHandlers) Sidebar(ctx context.Context) templ.Component {
	workspaces := h.workspaceOptions(ctx)
	active, err := h.uc.List.Execute(ctx, nil)
	if err != nil {
		active = nil
	}
	return components.BoardSidebar(state.BuildSidebar(workspaces, tasksToDTO(active, h.now())))
}
