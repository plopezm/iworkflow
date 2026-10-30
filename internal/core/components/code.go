package components

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

// LineMark marks a line as added, modified or deleted against the last saved version.
type LineMark string

// Line marks. The empty value is an unmarked line.
const (
	LineAdded    LineMark = "added"
	LineModified LineMark = "modified"
	LineDeleted  LineMark = "deleted"
)

// LineNumbers is the text of the gutter for text: "1\n2\n…", at least one line.
func LineNumbers(text string) string {
	lines := strings.Count(text, "\n") + 1
	numbers := make([]string, 0, lines)
	for n := 1; n <= lines; n++ {
		numbers = append(numbers, strconv.Itoa(n))
	}
	return strings.Join(numbers, "\n")
}

// MarkClass is the class of a line number according to its mark.
func MarkClass(mark LineMark) string {
	switch mark {
	case LineAdded:
		return "gm-added"
	case LineModified:
		return "gm-modified"
	case LineDeleted:
		return "gm-deleted"
	default:
		return "gm-none"
	}
}

// CodeViewProps configures the read-only, syntax-highlighted viewer.
type CodeViewProps struct {
	Text     string
	Filename string
}

// CodeEditorProps configures the editable viewer: a transparent textarea over a highlighted layer.
type CodeEditorProps struct {
	Name     string
	Value    string
	Filename string
	Disabled bool
	// Marks marks the lines of the gutter, one entry per line (empty: no marks).
	Marks []LineMark
	// Attrs carries the behaviour (e.g. hx-post on Ctrl+S via data-code-save).
	Attrs templ.Attributes
}

// codeAttributes are the attributes web/static/js/code.js reads to highlight the content.
func codeAttributes(filename string) templ.Attributes {
	return templ.Attributes{"data-code-filename": filename}
}

// splitNumbers returns the gutter numbers of text, one per line.
func splitNumbers(text string) []string {
	return strings.Split(LineNumbers(text), "\n")
}
