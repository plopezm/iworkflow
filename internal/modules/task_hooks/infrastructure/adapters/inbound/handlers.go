// Package inbound holds the HTTP adapter of the task_hooks module: the Hooks screen and the htmx endpoints that change it.
// Every response is the screen with status 200; the outcome of an action, a rejected hook included, is rendered inside
// it. A malformed form is the only 400. The screen is a dock tab (Page) and each action replaces it in place.
//
//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
	hookcomponents "iworkflow/internal/modules/task_hooks/presentation/components"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// runLimit bounds the run history: the newest runs that match the filter.
const runLimit = 200

// hookParam is the query key of the editor that opens a stored hook.
const hookParam = "hook"

// UseCases are the use cases the handlers call.
type UseCases struct {
	List             *application.ListHooks
	Projects         *application.ListProjects
	Save             *application.SaveHook
	Delete           *application.DeleteHook
	SaveAssignment   *application.SaveAssignment
	DeleteAssignment *application.DeleteAssignment
	Reorder          *application.ReorderAssignments
	Assignments      *application.ListWorkspaceAssignments
	Runs             *application.ListRuns
	Test             *application.TestHook
	Browse           *application.BrowsePath
}

// Handlers serves the Hooks screen and its actions.
type Handlers struct {
	uc UseCases
}

// NewHandlers builds the handlers over the use cases.
func NewHandlers(uc UseCases) *Handlers { return &Handlers{uc: uc} }

// Mount registers the endpoints. The screen itself is a dock tab (Page), so its address is not mounted here.
//
//	GET  /hooks/library                                     the library tab: the list of hooks
//	GET  /hooks/history                                     the run history, filtered by its query
//	GET  /hooks/group                                       the run order tab of a project, event and moment (query)
//	GET  /hooks/editor                                      the editor dialog: a new hook, a stored one (?hook=) or a changed kind
//	GET  /hooks/{hook}                                      the assignments view of the hook
//	GET  /hooks/{hook}/assignments/new                      the add-assignment dialog over the view
//	GET  /hooks/{hook}/test/new                             the test dialog over the view
//	POST /hooks                                             save the editor form (create, or edit when it carries an id)
//	POST /hooks/order                                       set the order of a group
//	POST /hooks/{hook}/delete                               delete the hook with its assignments
//	POST /hooks/{hook}/test                                 run the hook once for a project, event and moment
//	POST /hooks/{hook}/assignments                          assign the hook to a project, event and moment
//	POST /hooks/{hook}/assignments/{assignment}             save a stored assignment (the toggle posts it)
//	POST /hooks/{hook}/assignments/{assignment}/delete      remove an assignment
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/hooks/library", h.library)
	r.Get("/hooks/history", h.historyTab)
	r.Get("/hooks/group", h.groupTab)
	r.Get("/hooks/editor", h.editor)
	r.Get("/hooks/{hook}", h.open)
	r.Get("/hooks/{hook}/assignments/new", h.assignDialog)
	r.Get("/hooks/{hook}/test/new", h.testDialog)
	r.Post("/hooks", h.save)
	r.Post("/hooks/order", h.reorder)
	r.Post("/hooks/{hook}/delete", h.remove)
	r.Post("/hooks/{hook}/test", h.test)
	r.Post("/hooks/{hook}/assignments", h.addAssignment)
	r.Post("/hooks/{hook}/assignments/{assignment}", h.saveAssignment)
	r.Post("/hooks/{hook}/assignments/{assignment}/delete", h.removeAssignment)
}

// Page renders the Hooks screen as a dock tab: the library, with nothing open.
func (h *Handlers) Page(ctx context.Context) templ.Component {
	return hookcomponents.Screen(h.load(ctx, screenOptions{tab: state.TabLibrary}))
}

// screenOptions is what an answer shows: the tab, the outcome of the action, what is open and the forms as they were
// submitted, so a rejected form keeps what the user typed. A form that is set is a dialog that is open: the editor, the
// add-assignment form and the test form; a dialog closes by leaving its form unset.
type screenOptions struct {
	tab      state.Tab
	notice   state.Notice
	failure  string
	editor   *state.HookForm
	browser  *state.Browser
	selected string
	add      *state.AssignmentForm
	test     *state.AssignmentForm
	result   *state.TestResult
	group    dto.Group
	history  dto.History
}

// load builds the screen. A failure to read the library is shown in the screen, which then has no rows; a failure to
// read one section leaves the other sections in place.
func (h *Handlers) load(ctx context.Context, opts screenOptions) state.Screen {
	tab := opts.tab
	if tab == "" {
		tab = state.TabLibrary
	}
	screen := state.Screen{Tab: tab, Notice: opts.notice, Failure: opts.failure, Editor: opts.editor, Browser: opts.browser, Modal: modalOf(opts)}
	projects, err := h.uc.Projects.Execute(ctx)
	if err != nil {
		screen.Failure = firstFailure(screen.Failure, err)
	}
	names := projectNames(projects)
	screen.Projects = projectChoices(projects)
	items, err := h.uc.List.Execute(ctx)
	if err != nil {
		screen.Failure = firstFailure(screen.Failure, err)
		return withModalFailure(screen)
	}
	screen.Hooks = hookRows(items)
	if opts.selected != "" {
		screen.Selected, err = h.detail(opts, items, names)
		if err != nil {
			screen.Failure = firstFailure(screen.Failure, err)
		}
	}
	if tab == state.TabOrder {
		screen.Group = h.groupOf(ctx, opts.group, projects, &screen)
	}
	if tab == state.TabHistory {
		screen.History = h.historyOf(ctx, opts.history, items, projects, names, &screen)
	}
	return withModalFailure(screen)
}

// modalOf is the dialog the options open. The editor wins over the forms of a hook: an editor is never opened over them.
func modalOf(opts screenOptions) state.Modal {
	switch {
	case opts.editor != nil:
		return state.Modal{Kind: state.ModalEditor}
	case opts.add != nil:
		return state.Modal{Kind: state.ModalAssign}
	case opts.test != nil:
		return state.Modal{Kind: state.ModalTest}
	default:
		return state.Modal{}
	}
}

// withModalFailure settles the dialog and its failure. The failure moves into the dialog that is open, where the user is
// looking. A dialog of a hook needs the hook open: without it no dialog is shown, so the failure stays with the screen.
func withModalFailure(screen state.Screen) state.Screen {
	needsHook := screen.Modal.Kind == state.ModalAssign || screen.Modal.Kind == state.ModalTest
	if needsHook && screen.Selected == nil {
		screen.Modal = state.Modal{}
	}
	if screen.Modal.Kind == state.ModalNone {
		return screen
	}
	screen.Modal.Failure, screen.Failure = screen.Failure, ""
	return screen
}

// detail is the assignment editor of the selected hook. An unknown hook is hookerrors.ErrHookNotFound.
func (h *Handlers) detail(opts screenOptions, items []application.HookListItem, names map[string]string) (*state.HookDetail, error) {
	for _, item := range items {
		if item.Hook.ID() != opts.selected {
			continue
		}
		add := defaultAssignmentForm("")
		if opts.add != nil {
			add = *opts.add
		}
		test := defaultAssignmentForm("")
		if opts.test != nil {
			test = *opts.test
		}
		return hookDetail(item, names, add, test, opts.result), nil
	}
	return nil, hookerrors.ErrHookNotFound
}

// groupOf is the run order panel. Without a project chosen it shows the first project; without an event or a moment it
// shows the first of each.
func (h *Handlers) groupOf(ctx context.Context, requested dto.Group, projects []application.ProjectOption, screen *state.Screen) *state.Group {
	group := requested
	if group.Project == "" && len(projects) > 0 {
		group.Project = projects[0].ID
	}
	if group.Event == "" {
		group.Event = models.EventTaskStart
	}
	if group.Phase == "" {
		group.Phase = models.PhasePre
	}
	out := &state.Group{Project: group.Project, Event: string(group.Event), Phase: string(group.Phase)}
	if group.Project == "" {
		return out
	}
	assigned, err := h.uc.Assignments.Execute(ctx, group.Project)
	if err != nil {
		screen.Failure = firstFailure(screen.Failure, err)
		return out
	}
	out.Rows = groupRows(assigned, group)
	return out
}

// historyOf is the run history with its filter. A failure to read it is shown in the screen and the history has no runs.
func (h *Handlers) historyOf(ctx context.Context, filter dto.History, items []application.HookListItem, projects []application.ProjectOption, names map[string]string, screen *state.Screen) *state.History {
	out := &state.History{
		Failed:    filter.Failed,
		HookID:    filter.HookID,
		ProjectID: filter.ProjectID,
		Hooks:     hookChoices(items),
		Projects:  projectChoices(projects),
	}
	runs, err := h.uc.Runs.Execute(ctx, filter.RunFilter(runLimit))
	if err != nil {
		screen.Failure = firstFailure(screen.Failure, err)
		return out
	}
	out.Runs = runRows(runs, names)
	return out
}

// firstFailure keeps the first failure of the screen: the one that explains the most.
func firstFailure(current string, err error) string {
	if current != "" {
		return current
	}
	return err.Error()
}

// library is the library tab. The run order panel shows the project the request names.
func (h *Handlers) library(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, h.load(r.Context(), screenOptions{tab: state.TabLibrary}))
}

// historyTab is the run history, filtered by its query. Failed-only is on until an applied filter turns it off.
func (h *Handlers) historyTab(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, h.load(r.Context(), screenOptions{tab: state.TabHistory, history: dto.HistoryFromQuery(r.URL.Query())}))
}

// groupTab shows the run order tab of the project, event and moment the query names.
func (h *Handlers) groupTab(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, h.load(r.Context(), screenOptions{tab: state.TabOrder, group: dto.GroupFromForm(r.URL.Query())}))
}

// open shows the assignments view of one hook, in place of the library list.
func (h *Handlers) open(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, h.load(r.Context(), screenOptions{tab: state.TabLibrary, selected: chi.URLParam(r, "hook")}))
}

// assignDialog opens the add-assignment dialog over the assignments view of the hook, with a new assignment's values.
func (h *Handlers) assignDialog(w http.ResponseWriter, r *http.Request) {
	hookID := chi.URLParam(r, "hook")
	add := defaultAssignmentForm("")
	h.render(w, r, h.load(r.Context(), screenOptions{tab: state.TabLibrary, selected: hookID, add: &add}))
}

// testDialog opens the test dialog over the assignments view of the hook, with the default place of a test.
func (h *Handlers) testDialog(w http.ResponseWriter, r *http.Request) {
	hookID := chi.URLParam(r, "hook")
	test := defaultAssignmentForm("")
	h.render(w, r, h.load(r.Context(), screenOptions{tab: state.TabLibrary, selected: hookID, test: &test}))
}

// editor opens the editor. Without a kind in the query it starts from the stored hook (?hook=) or from a new one. With a
// kind, the query is the form as the user left it, so a change of kind keeps what was typed.
func (h *Handlers) editor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	opts := screenOptions{tab: state.TabLibrary}
	var form state.HookForm
	switch {
	case query.Has(dto.FieldKind):
		form = hookFormFromValues(query)
		opts.browser = h.browser(ctx, &form, query)
	case query.Get(hookParam) != "":
		form = defaultHookForm()
		hook, err := h.findHook(ctx, query.Get(hookParam))
		if err != nil {
			opts.failure = err.Error()
		} else {
			form = hookFormFromHook(hook)
		}
	default:
		form = defaultHookForm()
	}
	opts.editor = &form
	h.render(w, r, h.load(ctx, opts))
}

// browser applies a chosen path to the field being browsed, or lists the folder to show. It is nil when the request
// does not browse. The browser starts at the home folder. A folder that cannot be listed is
// shown as the error of the browser, not of the screen.
func (h *Handlers) browser(ctx context.Context, form *state.HookForm, query url.Values) *state.Browser {
	field := query.Get(state.BrowseField)
	var current *string
	switch field {
	case dto.FieldCommand:
		current = &form.Command
	case dto.FieldWorkingDir:
		current = &form.WorkingDir
	default:
		return nil
	}
	if pick := query.Get(state.PickField); pick != "" {
		*current = pick
		return nil
	}
	start := query.Get(state.BrowsePathField)
	if start == "" && field == dto.FieldCommand && *current != "" {
		start = filepath.Dir(*current)
	}
	if start == "" && field == dto.FieldWorkingDir {
		start = *current
	}
	browser := &state.Browser{Field: field, Files: field == dto.FieldCommand}
	listing, err := h.uc.Browse.Execute(ctx, application.BrowsePathInput{Path: start})
	if err != nil {
		browser.Path = start
		browser.Error = err.Error()
		return browser
	}
	browser.Path, browser.Parent = listing.Path, listing.Parent
	for _, entry := range listing.Entries {
		if entry.IsDir || browser.Files {
			browser.Entries = append(browser.Entries, state.BrowserEntry{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir})
		}
	}
	return browser
}

// save creates a hook, or edits the one the form names.
func (h *Handlers) save(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	input, err := dto.SaveHookFromForm(r.PostForm)
	notice := state.NoticeHookSaved
	if err == nil {
		if input.ID != "" {
			notice = state.NoticeHookUpdated
		}
		_, err = h.uc.Save.Execute(ctx, input)
	}
	if err != nil {
		// The editor stays open with the values as typed, so the user corrects them instead of typing them again.
		form := hookFormFromValues(r.PostForm)
		h.render(w, r, h.load(ctx, screenOptions{tab: state.TabLibrary, editor: &form, failure: err.Error()}))
		return
	}
	h.render(w, r, h.load(ctx, screenOptions{tab: state.TabLibrary, notice: notice}))
}

// reorder sets the whole order of one project, event and moment. The order must name each of its assignments once.
func (h *Handlers) reorder(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	group := dto.GroupFromForm(r.PostForm)
	opts := screenOptions{tab: state.TabOrder, notice: state.NoticeOrdered, group: group}
	if err := h.uc.Reorder.Execute(ctx, group.Project, group.Event, group.Phase, dto.ReorderFromForm(r.PostForm)); err != nil {
		opts.notice = state.NoticeNone
		opts.failure = err.Error()
	}
	h.render(w, r, h.load(ctx, opts))
}

// remove deletes a hook with its assignments. Its run records stay in the history.
func (h *Handlers) remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	opts := screenOptions{tab: state.TabLibrary, notice: state.NoticeHookDeleted}
	if err := h.uc.Delete.Execute(ctx, chi.URLParam(r, "hook")); err != nil {
		opts.notice = state.NoticeNone
		opts.failure = err.Error()
	}
	h.render(w, r, h.load(ctx, opts))
}

// test runs the hook once for the project, event and moment of the form. A run that could not start is shown with its
// reason. Any other refusal (an unknown hook, an unknown project) is the failure of the screen.
func (h *Handlers) test(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	hookID := chi.URLParam(r, "hook")
	chosen := assignmentFormFromValues(r.PostForm)
	opts := screenOptions{tab: state.TabLibrary, selected: hookID, test: &chosen}
	outcome, err := h.uc.Test.Execute(ctx, dto.TestHookFromForm(r.PostForm, hookID))
	switch {
	case errors.Is(err, hookerrors.ErrRunNotStarted), errors.Is(err, hookerrors.ErrRunCancelled):
		opts.result = &state.TestResult{Status: state.ResultError, ExitCode: -1, Failure: err.Error()}
	case err != nil:
		opts.failure = err.Error()
	default:
		opts.result = &state.TestResult{Status: resultStatus(outcome.Status), ExitCode: outcome.ExitCode, Output: outcome.Output}
	}
	h.render(w, r, h.load(ctx, opts))
}

// addAssignment assigns the hook to a project, event and moment. A repeated one is refused by the use case.
func (h *Handlers) addAssignment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	hookID := chi.URLParam(r, "hook")
	chosen := assignmentFormFromValues(r.PostForm)
	opts := screenOptions{tab: state.TabLibrary, selected: hookID, notice: state.NoticeAssigned}
	if _, err := h.uc.SaveAssignment.Execute(ctx, dto.SaveAssignmentFromForm(r.PostForm, hookID)); err != nil {
		opts.notice = state.NoticeNone
		opts.failure = err.Error()
		opts.add = &chosen
	}
	h.render(w, r, h.load(ctx, opts))
}

// saveAssignment saves a stored assignment. The toggle posts its values with the new enabled flag.
func (h *Handlers) saveAssignment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	hookID := chi.URLParam(r, "hook")
	input := dto.SaveAssignmentFromForm(r.PostForm, hookID)
	input.ID = chi.URLParam(r, "assignment")
	opts := screenOptions{tab: state.TabLibrary, selected: hookID, notice: state.NoticeAssignmentUpdated}
	if _, err := h.uc.SaveAssignment.Execute(ctx, input); err != nil {
		opts.notice = state.NoticeNone
		opts.failure = err.Error()
	}
	h.render(w, r, h.load(ctx, opts))
}

// removeAssignment detaches the hook from a project, event and moment. The hook stays in the library.
func (h *Handlers) removeAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hookID := chi.URLParam(r, "hook")
	opts := screenOptions{tab: state.TabLibrary, selected: hookID, notice: state.NoticeAssignmentDeleted}
	if err := h.uc.DeleteAssignment.Execute(ctx, chi.URLParam(r, "assignment")); err != nil {
		opts.notice = state.NoticeNone
		opts.failure = err.Error()
	}
	h.render(w, r, h.load(ctx, opts))
}

// findHook returns the stored hook with the id. An unknown id is hookerrors.ErrHookNotFound.
func (h *Handlers) findHook(ctx context.Context, hookID string) (models.Hook, error) {
	items, err := h.uc.List.Execute(ctx)
	if err != nil {
		return models.Hook{}, fmt.Errorf("find hook: %w", err)
	}
	for _, item := range items {
		if item.Hook.ID() == hookID {
			return item.Hook, nil
		}
	}
	return models.Hook{}, hookerrors.ErrHookNotFound
}

// render answers the screen. The fragment carries its own outcome, so the response is always 200.
func (h *Handlers) render(w http.ResponseWriter, r *http.Request, view state.Screen) {
	web.Render(w, r, http.StatusOK, hookcomponents.Screen(view))
}
