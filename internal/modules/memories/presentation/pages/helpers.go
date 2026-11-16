package pages

import (
	"html"

	"iworkflow/internal/core/markdown"
)

// markdownHTML renders memory content as sanitized HTML. Raw HTML in it is shown as escaped text and unsafe links are
// neutralized. Content that cannot be rendered is shown as escaped text.
func markdownHTML(text string) string {
	rendered, err := markdown.Render(text)
	if err != nil {
		return html.EscapeString(text)
	}
	return rendered
}
