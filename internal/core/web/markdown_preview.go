package web

import (
	"net/http"

	"iworkflow/internal/core/components"
	"iworkflow/internal/core/markdown"
)

// MarkdownPreview renders the Markdown of the form field "content" as sanitized HTML, for the Code | Rendered switch of
// the file editors. It reads nothing from disk and changes nothing.
func MarkdownPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	Render(w, r, http.StatusOK, components.MarkdownBody(markdown.HTML(r.PostFormValue("content"))))
}
