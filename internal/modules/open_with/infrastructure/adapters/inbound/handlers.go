// Package inbound holds the HTTP adapter of the open_with module: the htmx fragment of the "Open with"
// dialog of a task. Every response is the dialog with status 200; the outcome of an action, an error included,
// is rendered inside it. A malformed form is the only 400.
package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/open_with/application"
	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/open_with/presentation/components"
	"iworkflow/internal/modules/open_with/presentation/state"
)

// UseCases are the use cases the handlers call.
type UseCases struct {
	List   *application.ListLaunchers
	Create *application.CreateLauncher
	Delete *application.DeleteLauncher
	Launch *application.Launch
	Browse *application.BrowseFiles
}

// Handlers serves the dialog.
type Handlers struct {
	uc UseCases
}

// NewHandlers builds the handlers over the use cases.
func NewHandlers(uc UseCases) *Handlers { return &Handlers{uc: uc} }

// Mount registers the endpoints.
//
//	GET  /open-with/tasks/{id}                              the dialog
//	POST /open-with/tasks/{id}                              add an application (form)
//	GET  /open-with/tasks/{id}/new                          the dialog with the add form open
//	GET  /open-with/tasks/{id}/browse                       the dialog with the file picker (dir, filter + the form values)
//	GET  /open-with/tasks/{id}/choose                       the dialog without the picker (file fills the path field)
//	POST /open-with/tasks/{id}/launchers/{launcher}/launch  open the worktree with the application
//	POST /open-with/tasks/{id}/launchers/{launcher}/delete  remove the application
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/open-with/tasks/{id}", h.show)
	r.Post("/open-with/tasks/{id}", h.create)
	r.Get("/open-with/tasks/{id}/new", h.showNew)
	r.Get("/open-with/tasks/{id}/browse", h.browse)
	r.Get("/open-with/tasks/{id}/choose", h.choose)
	r.Post("/open-with/tasks/{id}/launchers/{launcher}/launch", h.launch)
	r.Post("/open-with/tasks/{id}/launchers/{launcher}/delete", h.remove)
}

func (h *Handlers) show(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, state.DialogView{}, "", nil)
}

func (h *Handlers) showNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, state.DialogView{Adding: true}, "", nil)
}

// enteredFrom reads the add form values the picker requests carry.
func enteredFrom(query url.Values) state.Entered {
	return state.Entered{
		Name: query.Get(dto.FieldName), Path: query.Get(dto.FieldPath),
		Args: query.Get(dto.FieldArgs), Env: query.Get(dto.FieldEnv),
	}
}

// browse opens the picker. Without a folder it starts where the typed path is, or in the home folder.
func (h *Handlers) browse(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	entered := enteredFrom(query)
	filter := query.Get("filter")
	dir := query.Get("dir")
	start := dir
	if start == "" {
		start = entered.Path // the picker opens where the typed path is
	}
	picker := h.pickerAt(r.Context(), start, filter)
	if dir == "" && picker.Error != "" {
		picker = h.pickerAt(r.Context(), "", filter) // the typed path leads nowhere: start at home
	}
	h.render(w, r, state.DialogView{Adding: true, Entered: entered, Picker: &picker}, "", nil)
}

// choose closes the picker; a file fills the path field, no file keeps what was typed.
func (h *Handlers) choose(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	entered := enteredFrom(query)
	if file := query.Get("file"); file != "" {
		entered.Path = file
	}
	h.render(w, r, state.DialogView{Adding: true, Entered: entered}, "", nil)
}

// pickerAt lists a folder for the picker; a failure is shown inside it.
func (h *Handlers) pickerAt(ctx context.Context, dir string, filter string) state.Picker {
	picker := state.Picker{Dir: dir, Filter: filter}
	listing, err := h.uc.Browse.Execute(ctx, dir, filter)
	if err != nil {
		picker.Error = err.Error()
		return picker
	}
	picker.Dir, picker.Parent, picker.Truncated = listing.Path, listing.Parent, listing.Truncated
	for _, entry := range listing.Entries {
		picker.Entries = append(picker.Entries, state.PickerEntry{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir})
	}
	return picker
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	entered := state.Entered{
		Name: r.PostForm.Get(dto.FieldName), Path: r.PostForm.Get(dto.FieldPath),
		Args: r.PostForm.Get(dto.FieldArgs), Env: r.PostForm.Get(dto.FieldEnv),
	}
	input, err := dto.CreateLauncherFromForm(r.PostForm)
	if err == nil {
		_, err = h.uc.Create.Execute(r.Context(), input)
	}
	if err != nil {
		h.render(w, r, state.DialogView{Adding: true, Entered: entered}, "", err)
		return
	}
	h.render(w, r, state.DialogView{}, "", nil)
}

func (h *Handlers) launch(w http.ResponseWriter, r *http.Request) {
	launcher, err := h.uc.Launch.Execute(r.Context(), application.LaunchInput{
		TaskID: chi.URLParam(r, "id"), LauncherID: chi.URLParam(r, "launcher"),
	})
	if err != nil {
		h.render(w, r, state.DialogView{}, "", err)
		return
	}
	h.render(w, r, state.DialogView{}, launcher.Name(), nil)
}

func (h *Handlers) remove(w http.ResponseWriter, r *http.Request) {
	err := h.uc.Delete.Execute(r.Context(), chi.URLParam(r, "launcher"))
	if errors.Is(err, openerrors.ErrLauncherNotFound) {
		err = nil // already gone: the refreshed list says so
	}
	h.render(w, r, state.DialogView{}, "", err)
}

// render fills the task, the scope and the list in the view and answers the dialog.
func (h *Handlers) render(w http.ResponseWriter, r *http.Request, view state.DialogView, started string, failure error) {
	view.TaskID = chi.URLParam(r, "id")
	view.Scope = r.URL.Query().Get("scope")
	view.Started = started
	if failure != nil {
		view.Failure = failure.Error()
	}
	launchers, err := h.uc.List.Execute(r.Context())
	if err != nil && view.Failure == "" {
		view.Failure = err.Error()
	}
	for _, launcher := range launchers {
		view.Rows = append(view.Rows, state.Row{
			ID: launcher.ID(), Name: launcher.Name(), Path: launcher.Path(),
			Args: len(launcher.Args()), EnvVars: len(launcher.Env()),
		})
	}
	web.Render(w, r, http.StatusOK, components.Dialog(view)) //nolint:contextcheck // templ components receive the request context at Render time
}
