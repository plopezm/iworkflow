package web

import (
	"errors"
	"net/http"

	"iworkflow/internal/core/components"
	"iworkflow/internal/core/markdown"
)

// MarkdownPreviewMaxBytes is the largest text the preview renders. The route has its own limit, whatever the
// middleware of the router sets, because rendering costs CPU in proportion to the text.
const MarkdownPreviewMaxBytes int64 = 1024 * 1024

// MarkdownPreview renders the Markdown of the form field "content" as sanitized HTML, for the Code | Rendered switch of
// the file editors. It reads nothing from disk and changes nothing.
func MarkdownPreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MarkdownPreviewMaxBytes)
	if err := r.ParseForm(); err != nil {
		status := http.StatusBadRequest
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, "invalid form", status)
		return
	}
	Render(w, r, http.StatusOK, components.MarkdownBody(markdown.HTML(r.PostFormValue("content"))))
}
