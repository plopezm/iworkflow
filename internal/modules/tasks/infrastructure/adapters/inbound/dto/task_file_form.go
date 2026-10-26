package dto

import (
	"errors"
	"fmt"
	"net/http"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskFileBodyLimit caps the body of a file request. Form encoding turns a three-byte UTF-8 character into nine bytes,
// so the largest allowed file fits three times over, plus a margin for the other fields.
const TaskFileBodyLimit int64 = 3*models.MaxTaskFileBytes + 64*1024

// ErrTaskFileTooLarge reports a file request whose content, or whose body, is above the limit of a task file.
var ErrTaskFileTooLarge = errors.New("task file too large")

// TaskFileForm is a request about one file of a task worktree: the scope the file is shown in, its path relative to
// the worktree and, for a save, the new content and the digest of the content the editor loaded.
type TaskFileForm struct {
	Scope   string
	Path    string
	Content string
	Base    string
}

// DecodeTaskFileForm reads the fields of a file request, from the query string or the form. The body is capped before
// it is read; content above models.MaxTaskFileBytes is refused here as well, so the use case never sees it.
func DecodeTaskFileForm(w http.ResponseWriter, r *http.Request) (TaskFileForm, error) {
	r.Body = http.MaxBytesReader(w, r.Body, TaskFileBodyLimit)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return TaskFileForm{}, ErrTaskFileTooLarge
		}
		return TaskFileForm{}, fmt.Errorf("parse the file form: %w", err)
	}
	form := TaskFileForm{
		Scope:   r.Form.Get("scope"),
		Path:    r.Form.Get("path"),
		Content: r.Form.Get("content"),
		Base:    r.Form.Get("base"),
	}
	if len(form.Content) > models.MaxTaskFileBytes {
		return TaskFileForm{}, ErrTaskFileTooLarge
	}
	return form, nil
}
