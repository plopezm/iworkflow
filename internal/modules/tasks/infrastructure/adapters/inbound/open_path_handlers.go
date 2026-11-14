package inbound

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/i18n"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// OpenPathRoute opens a path of the disk that the agent wrote in the chat. POST, form field "path".
const OpenPathRoute = "/tasks/{id}/open-path"

// maxOpenPathBytes bounds the body of the request: a path is short.
const maxOpenPathBytes int64 = 8 << 10

// Texts of the open-path endpoint. The page script shows them as the tooltip of the link that failed.
var (
	msgOpenPathInvalid = i18n.Msg{
		En: "This path cannot be opened.",
		Es: "No se puede abrir esta ruta.",
		Zh: "无法打开此路径。",
		De: "Dieser Pfad kann nicht geöffnet werden.",
		Fr: "Ce chemin ne peut pas être ouvert.",
	}
	msgOpenPathBlocked = i18n.Msg{
		En: "Programs and scripts are not opened from the chat.",
		Es: "Los programas y scripts no se abren desde el chat.",
		Zh: "不会从聊天中打开程序和脚本。",
		De: "Programme und Skripte werden nicht aus dem Chat geöffnet.",
		Fr: "Les programmes et scripts ne s’ouvrent pas depuis le chat.",
	}
	msgOpenPathMissing = i18n.Msg{
		En: "This path does not exist.",
		Es: "Esta ruta no existe.",
		Zh: "此路径不存在。",
		De: "Dieser Pfad existiert nicht.",
		Fr: "Ce chemin n’existe pas.",
	}
	msgOpenPathFailed = i18n.Msg{
		En: "The path could not be opened.",
		Es: "No se pudo abrir la ruta.",
		Zh: "无法打开该路径。",
		De: "Der Pfad konnte nicht geöffnet werden.",
		Fr: "Le chemin n’a pas pu être ouvert.",
	}
)

// PathOpenerUseCase opens a path of the chat of a task. *application.OpenTaskPath satisfies it.
type PathOpenerUseCase interface {
	Execute(ctx context.Context, id models.TaskID, raw string) error
}

// OpenPathHandlers serves the links to the disk of the chat.
type OpenPathHandlers struct {
	uc PathOpenerUseCase
}

// NewOpenPathHandlers builds the handlers over the use case.
func NewOpenPathHandlers(uc PathOpenerUseCase) *OpenPathHandlers {
	return &OpenPathHandlers{uc: uc}
}

// Mount registers the endpoint.
func (h *OpenPathHandlers) Mount(r chi.Router) {
	r.Post(OpenPathRoute, h.open)
}

// open answers 204 when the path was handed to the system, and a status with a plain text reason otherwise.
func (h *OpenPathHandlers) open(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, msgInvalidTask.T(ctx), http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxOpenPathBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, msgOpenPathInvalid.T(ctx), http.StatusBadRequest)
		return
	}
	if err := h.uc.Execute(ctx, id, r.PostForm.Get("path")); err != nil {
		status, message := openPathFailure(ctx, err)
		http.Error(w, message, status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// openPathFailure maps a failure to its status and its message. Unknown failures are logged, never shown.
func openPathFailure(ctx context.Context, err error) (int, string) {
	switch {
	case errors.Is(err, werrors.NotFound("")):
		return http.StatusNotFound, msgInvalidTask.T(ctx)
	case errors.Is(err, werrors.ErrOpenPathInvalid):
		return http.StatusUnprocessableEntity, msgOpenPathInvalid.T(ctx)
	case errors.Is(err, werrors.ErrOpenPathBlocked):
		return http.StatusForbidden, msgOpenPathBlocked.T(ctx)
	case errors.Is(err, werrors.ErrOpenPathMissing):
		return http.StatusNotFound, msgOpenPathMissing.T(ctx)
	}
	slog.Error("could not open a path of the chat", "error", err)
	return http.StatusInternalServerError, msgOpenPathFailed.T(ctx)
}
