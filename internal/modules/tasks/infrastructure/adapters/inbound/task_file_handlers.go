//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// DiffSavedEvent is the HX-Trigger event a committed save sends. The diff view listens to it and reloads itself.
const DiffSavedEvent = state.DiffSavedEvent

// TaskFileRoute is the editor of a file of a task: GET loads it, POST saves it.
const TaskFileRoute = "/tasks/{id}/file"

// TaskFileStateRoute answers whether the editor holds unsaved changes. It runs while the text is typed.
const TaskFileStateRoute = "/tasks/{id}/file/state"

// TaskFileCloseRoute closes the editor of a file without saving it.
const TaskFileCloseRoute = "/tasks/{id}/file/close"

// Texts of the file endpoints. Domain errors are mapped to these, so no internal text reaches the user.
var (
	msgInvalidPath = i18n.Msg{
		En: "The file path is not valid.",
		Es: "La ruta del fichero no es válida.",
		Zh: "文件路径无效。",
		De: "Der Dateipfad ist ungültig.",
		Fr: "Le chemin du fichier n’est pas valide.",
	}
	msgFileTooLarge = i18n.Msg{
		En: "The file is larger than 1 MiB and cannot be saved here.",
		Es: "El fichero supera 1 MiB y no se puede guardar desde aquí.",
		Zh: "文件超过 1 MiB，无法在此保存。",
		De: "Die Datei ist größer als 1 MiB und kann hier nicht gespeichert werden.",
		Fr: "Le fichier dépasse 1 MiB et ne peut pas être enregistré ici.",
	}
	msgFileNotEditable = i18n.Msg{
		En: "This file cannot be edited.",
		Es: "Este fichero no se puede editar.",
		Zh: "该文件不能编辑。",
		De: "Diese Datei kann nicht bearbeitet werden.",
		Fr: "Ce fichier ne peut pas être modifié.",
	}
	msgFileUnavailable = i18n.Msg{
		En: "The file could not be read.",
		Es: "No se pudo leer el fichero.",
		Zh: "无法读取文件。",
		De: "Die Datei konnte nicht gelesen werden.",
		Fr: "Le fichier n’a pas pu être lu.",
	}
	msgFileSaveFailed = i18n.Msg{
		En: "The file could not be saved.",
		Es: "No se pudo guardar el fichero.",
		Zh: "无法保存文件。",
		De: "Die Datei konnte nicht gespeichert werden.",
		Fr: "Le fichier n’a pas pu être enregistré.",
	}
)

// TaskFileReader reads a file of the worktree of a task. *application.GetTaskFile satisfies it.
type TaskFileReader interface {
	Execute(ctx context.Context, id models.TaskID, rel string) (models.TaskFileContent, error)
}

// TaskFileWriter saves a file of the worktree of a task and commits it. It reports whether a commit was created.
// *application.SaveTaskFile satisfies it.
type TaskFileWriter interface {
	Execute(ctx context.Context, id models.TaskID, rel string, content string) (bool, error)
}

// TaskFileUseCases are what the file editor reads and saves.
type TaskFileUseCases struct {
	Read  TaskFileReader
	Write TaskFileWriter
}

// TaskFileHandlers serves the editor of a file of a task from the diff view.
type TaskFileHandlers struct {
	uc TaskFileUseCases
}

// NewTaskFileHandlers builds the handlers over the use cases.
func NewTaskFileHandlers(uc TaskFileUseCases) *TaskFileHandlers {
	return &TaskFileHandlers{uc: uc}
}

// Mount registers the editor endpoints. The diff view answers them with its file cards.
func (h *TaskFileHandlers) Mount(r chi.Router) {
	r.Get(TaskFileRoute, h.editor)
	r.Post(TaskFileRoute, h.save)
	r.Post(TaskFileStateRoute, h.state)
	r.Get(TaskFileCloseRoute, h.close)
}

// editor loads a file into the editor of its card.
func (h *TaskFileHandlers) editor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, form, err := h.request(w, r)
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(requestText(ctx, err)))
		return
	}
	if form.Path == "" {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidPath.T(ctx)))
		return
	}
	file, err := h.uc.Read.Execute(ctx, id, form.Path)
	if err != nil {
		web.Render(w, r, http.StatusOK, h.failure(ctx, err, msgFileUnavailable))
		return
	}
	view := state.FileEditor{
		Scope:   form.Scope,
		TaskID:  id.String(),
		Path:    form.Path,
		Content: file.Content,
		Digest:  state.ContentDigest(file.Content),
		EOL:     state.LineEndingOf(file.Content),
	}
	web.Render(w, r, http.StatusOK, components.TaskFileEditor(view))
}

// save writes the file and commits it. A commit closes the editor and asks the diff view to reload. When nothing
// changed, the editor stays open and says so. The file gets the line ending it had, and a save above the size limit
// answers the editor with the text that was typed.
//
// The save does not compare the file with the digest the editor loaded (Base): a change that the file received after
// the editor was opened is overwritten. Rust SaveTaskFile does the same, so the check is a product decision, not a
// parity fix.
func (h *TaskFileHandlers) save(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, form, err := h.request(w, r)
	if errors.Is(err, dto.ErrTaskFileTooLarge) && form.Path != "" {
		web.Render(w, r, http.StatusOK, components.TaskFileEditor(state.FileEditor{
			Scope:   form.Scope,
			TaskID:  id.String(),
			Path:    form.Path,
			Content: form.Content,
			Digest:  form.Base,
			EOL:     form.EOL,
			Error:   msgFileTooLarge.T(ctx),
		}))
		return
	}
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(requestText(ctx, err)))
		return
	}
	if form.Path == "" {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidPath.T(ctx)))
		return
	}
	editor := state.FileEditor{
		Scope:   form.Scope,
		TaskID:  id.String(),
		Path:    form.Path,
		Content: form.Content,
		Digest:  form.Base,
		EOL:     form.EOL,
	}
	committed, err := h.uc.Write.Execute(ctx, id, form.Path, state.ApplyLineEnding(form.Content, form.EOL))
	if err != nil {
		if errors.Is(err, werrors.NotFound("")) {
			web.Render(w, r, http.StatusOK, components.TaskGone())
			return
		}
		editor.Error = fileErrorText(ctx, err, msgFileSaveFailed)
		web.Render(w, r, http.StatusOK, components.TaskFileEditor(editor))
		return
	}
	if committed {
		w.Header().Set("HX-Trigger", DiffSavedEvent)
		web.Render(w, r, http.StatusOK, templ.NopComponent)
		return
	}
	editor.Unchanged = true
	web.Render(w, r, http.StatusOK, components.TaskFileEditor(editor))
}

// state answers the status line of the editor: whether the text differs from the digest of what was loaded.
func (h *TaskFileHandlers) state(w http.ResponseWriter, r *http.Request) {
	_, form, err := h.request(w, r)
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(requestText(r.Context(), err)))
		return
	}
	web.Render(w, r, http.StatusOK, components.TaskFileStatus(state.IsDirty(form.Content, form.Base)))
}

// close answers the editor slot with nothing, which closes the editor without touching the file.
func (h *TaskFileHandlers) close(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, templ.NopComponent)
}

// request reads the task and the fields of a file request. It returns the error of a request that cannot be read: an
// invalid task, or a form that is not valid. A form above the size limit comes back with its fields, so a save can show
// the text again.
func (h *TaskFileHandlers) request(w http.ResponseWriter, r *http.Request) (models.TaskID, dto.TaskFileForm, error) {
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		return models.TaskID{}, dto.TaskFileForm{}, errInvalidTaskRequest
	}
	form, err := dto.DecodeTaskFileForm(w, r)
	if err != nil {
		return id, form, fmt.Errorf("read the file request: %w", err)
	}
	return id, form, nil
}

// errInvalidTaskRequest is the error of a request that names no valid task.
var errInvalidTaskRequest = errors.New("invalid task in the file request")

// requestText is the message of a request that cannot be read, in the request language.
func requestText(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, errInvalidTaskRequest):
		return msgInvalidTask.T(ctx)
	case errors.Is(err, dto.ErrTaskFileTooLarge):
		return msgFileTooLarge.T(ctx)
	}
	return msgInvalidPath.T(ctx)
}

// failure answers why a file cannot be read: a task that no longer exists says so, and any other failure is logged and
// answered in the request language.
func (h *TaskFileHandlers) failure(ctx context.Context, err error, fallback i18n.Msg) templ.Component {
	if errors.Is(err, werrors.NotFound("")) {
		return components.TaskGone()
	}
	return shared.ErrorText(fileErrorText(ctx, err, fallback))
}

// fileErrorText is the message of a file failure in the request language. The error itself is logged, not shown.
func fileErrorText(ctx context.Context, err error, fallback i18n.Msg) string {
	switch {
	case errors.Is(err, werrors.InvalidFilePath("")):
		return msgInvalidPath.T(ctx)
	case errors.Is(err, werrors.FileNotEditable("")), errors.Is(err, werrors.InvalidState("", "")):
		return msgFileNotEditable.T(ctx)
	}
	slog.Error("could not use a file of the task", "error", err)
	return fallback.T(ctx)
}
