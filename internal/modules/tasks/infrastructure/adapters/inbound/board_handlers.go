//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"fmt"
	"log/slog"
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
	TaskEditRouteName       = "task_edit"
)

// boardRefreshEvent is the htmx event the list refreshes on after a create (see BoardList).
const boardRefreshEvent = "board:refresh"

// BoardUseCases are what the board reads and changes. The workspaces come through the outbound lookup, so the
// presentation never reaches another module.
type BoardUseCases struct {
	List            *application.ListTasks
	ListDone        *application.ListDoneTasks
	Get             *application.GetTask
	CreateLinked    *application.CreateLinkedTask
	Update          *application.UpdateTask
	Start           *application.StartTask
	Delete          *application.DeleteTask
	ListAttachments *application.ListTaskAttachments
	ListBranches    *application.ListWorkspaceBranches
	Workspaces      outbound.WorkspaceLookup
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
			Name:    TaskEditRouteName,
			Pattern: "/tasks/{id}/edit",
			Icon:    "●",
			Title: func(ctx context.Context, _ dock.Params) string {
				return components.EditTitle(ctx)
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.editTab(ctx, params["id"])
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

// Mount registers the htmx endpoints of the board: the list, the sidebar, the create dialog and the actions of
// a backlog card, and the save of the edit page.
func (h *BoardHandlers) Mount(r chi.Router) {
	r.Get("/board/list", h.list)
	r.Get("/board/sidebar", h.sidebar)
	r.Get("/board/tasks/new", h.newTask)
	r.Get("/board/tasks/new/close", h.closeNewTask)
	r.Get("/board/tasks/branches", h.branches)
	r.Post("/board/tasks", h.createTask)
	r.Post("/board/tasks/{id}/start", h.startTask)
	r.Post("/board/tasks/{id}/delete", h.deleteTask)
	r.Post("/tasks/{id}/update", h.updateTask)
}

// title is the tab title of the board of the requested workspace; an unknown one keeps the plain name.
func (h *BoardHandlers) title(ctx context.Context, requested string) string {
	// A failed read is already logged by workspaceOptions; the tab then keeps the plain name.
	workspaces, err := h.workspaceOptions(ctx)
	if err != nil {
		return components.BoardTitle(ctx, "")
	}
	choice := state.ChooseWorkspace(requested, workspaces)
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

// newTask opens the create dialog, with the workspace the board shows selected. A link in the address (link_kind
// and link_anchor) makes the new task a successor of the anchor, as the graph asks for it.
func (h *BoardHandlers) newTask(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	input := dto.TaskForm{
		WorkspaceID: query.Get("workspace"),
		Priority:    state.DefaultPriority,
		LinkKind:    query.Get("link_kind"),
		LinkAnchor:  query.Get("link_anchor"),
	}
	web.Render(w, r, http.StatusOK, components.CreateTaskDialog(h.createForm(r.Context(), input, "")))
}

// closeNewTask closes the create dialog without creating anything.
func (h *BoardHandlers) closeNewTask(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, components.CreateTaskResult("", nil))
}

// createTask creates the task from the dialog, linked when the form says so. A form that does not validate, or a
// create that fails, keeps the dialog open with what was typed and the error. A created task closes the dialog,
// refreshes the list and shows what failed after the creation: its start and its dependencies.
func (h *BoardHandlers) createTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	form, err := dto.ParseTaskForm(w, r)
	if err != nil {
		web.Render(w, r, http.StatusOK, components.CreateTaskDialog(h.createForm(ctx, dto.TaskForm{}, err.Error())))
		return
	}
	newTask, link, err := form.ToNewTask()
	if err != nil {
		web.Render(w, r, http.StatusOK, components.CreateTaskDialog(h.createForm(ctx, form, err.Error())))
		return
	}
	result, err := h.uc.CreateLinked.Execute(ctx, newTask, link)
	if err != nil {
		web.Render(w, r, http.StatusOK, components.CreateTaskDialog(h.createForm(ctx, form, err.Error())))
		return
	}
	startError := ""
	if result.Created.StartError != nil {
		startError = result.Created.StartError.Error()
	}
	linkErrors := make([]string, 0, len(result.LinkErrors))
	for _, linkErr := range result.LinkErrors {
		linkErrors = append(linkErrors, linkErr.Error())
	}
	web.Trigger(w, boardRefreshEvent)
	web.Render(w, r, http.StatusOK, components.CreateTaskResult(startError, linkErrors))
}

// createForm rebuilds the dialog state from what was typed, with the error to show. A failed workspace read
// replaces the message: without the workspaces the dialog cannot be filled in or submitted.
func (h *BoardHandlers) createForm(ctx context.Context, input dto.TaskForm, message string) state.CreateForm {
	workspaces, err := h.workspaceOptions(ctx)
	if err != nil {
		message = err.Error()
	}
	branches, branchErr := h.branchesOf(ctx, input.WorkspaceID)
	if branchErr != nil && message == "" {
		message = branchErr.Error()
	}
	return state.CreateForm{
		Workspaces:    workspaces,
		WorkspaceID:   input.WorkspaceID,
		Title:         input.Title,
		Prompt:        input.Prompt,
		Priority:      priorityOrDefault(input.Priority),
		BaseBranch:    input.BaseBranch,
		DefaultBranch: defaultBranchOf(workspaces, input.WorkspaceID),
		Branches:      branches,
		MemoryLimit:   input.MemoryLimit,
		CPULimit:      input.CPULimit,
		ShowAdvanced:  input.ShowAdvanced,
		StartNow:      input.StartNow,
		LinkKind:      input.LinkKind,
		LinkAnchor:    input.LinkAnchor,
		Error:         message,
	}
}

// view builds what the list shows for the requested workspace. A failed read is reported in LoadError and
// leaves the other data empty, so the page still renders.
func (h *BoardHandlers) view(ctx context.Context, requested string, showCompleted bool, donePages int) state.BoardView {
	workspaces, err := h.workspaceOptions(ctx)
	if err != nil {
		return state.BoardView{Choice: state.WorkspaceChoice{Kind: state.ChoiceEmpty}, ShowCompleted: showCompleted, Pages: donePages, LoadError: err.Error()}
	}
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
		slog.Error("could not load the tasks of the board", "error", err)
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

// workspaceOptions lists the workspaces the board can show, as the switcher needs them. A failed read is
// logged and returned, so the page shows the error instead of a board with no workspaces.
func (h *BoardHandlers) workspaceOptions(ctx context.Context) ([]dto.WorkspaceOptionDTO, error) {
	refs, err := h.uc.Workspaces.List(ctx)
	if err != nil {
		slog.Error("could not list the workspaces of the board", "error", err)
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	options := make([]dto.WorkspaceOptionDTO, 0, len(refs))
	for _, ref := range refs {
		options = append(options, dto.WorkspaceOptionDTO{ID: ref.ID, Name: ref.Name, BaseBranch: ref.BaseBranch})
	}
	return options, nil
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
// read is shown in the sidebar, and the rows that could be read are still listed.
func (h *BoardHandlers) Sidebar(ctx context.Context) templ.Component {
	workspaces, workspaceErr := h.workspaceOptions(ctx)
	active, taskErr := h.uc.List.Execute(ctx, nil)
	if taskErr != nil {
		slog.Error("could not load the tasks of the sidebar", "error", taskErr)
		active = nil
	}
	rows := state.BuildSidebar(workspaces, tasksToDTO(active, h.now()))
	rows.LoadError = firstErrorText(workspaceErr, taskErr)
	return components.BoardSidebar(rows)
}

// firstErrorText is the message of the first error that is set, or empty when none is.
func firstErrorText(errs ...error) string {
	for _, err := range errs {
		if err != nil {
			return err.Error()
		}
	}
	return ""
}

// defaultBranchOf is the base branch of the workspace, or empty when the workspace is not listed.
func defaultBranchOf(workspaces []dto.WorkspaceOptionDTO, workspaceID string) string {
	for _, workspace := range workspaces {
		if workspace.ID == workspaceID {
			return workspace.BaseBranch
		}
	}
	return ""
}

// branchesOf lists the branches a task of the workspace can start from. No workspace chosen means no branches.
func (h *BoardHandlers) branchesOf(ctx context.Context, workspaceID string) ([]dto.BranchOptionDTO, error) {
	if workspaceID == "" {
		return nil, nil
	}
	branches, err := h.uc.ListBranches.Execute(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	options := make([]dto.BranchOptionDTO, 0, len(branches))
	for _, branch := range branches {
		options = append(options, dto.BranchOptionFromDomain(branch))
	}
	return options, nil
}
