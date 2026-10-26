package state

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// Rows the inline editor of a file shows: it grows with its content between these bounds, then scrolls.
const (
	editorMinRows int = 8
	editorMaxRows int = 28
)

// DiffSavedEvent is the event the diff view listens to: a committed save reloads it, so the file shows its new state.
const DiffSavedEvent = "iwf-diff-saved"

// The line endings of a file. A textarea holds LF only, so the editor remembers the ending of the file it loaded and
// the save writes it back.
const (
	LineEndingLF   = "lf"
	LineEndingCRLF = "crlf"
)

// FileEditor is the inline editor of one file of the diff view: the content it shows, the digest of the content it
// loaded (dirty tracking compares against it), the line ending of that file and the outcome of the last save.
type FileEditor struct {
	Scope     string
	TaskID    string
	Path      string
	Content   string
	Digest    string
	EOL       string
	Error     string
	Unchanged bool
}

// ContentDigest is the digest the editor keeps of the content it loaded. It is sent back in the form, so the server
// can tell whether the editor holds changes without keeping any state of its own. Line endings do not count: the
// browser and the file may disagree on them without any change of the text.
func ContentDigest(content string) string {
	sum := sha256.Sum256([]byte(withLF(content)))
	return hex.EncodeToString(sum[:])
}

// LineEndingOf is the line ending of a file: CRLF when the file has one, LF otherwise. A file with both endings is
// saved with CRLF on every line, because the textarea cannot hold a mix.
func LineEndingOf(content string) string {
	if strings.Contains(content, "\r\n") {
		return LineEndingCRLF
	}
	return LineEndingLF
}

// ApplyLineEnding returns the content with the line ending of a file. Text that already holds CRLF is not doubled.
func ApplyLineEnding(content string, eol string) string {
	normalized := withLF(content)
	if eol == LineEndingCRLF {
		return strings.ReplaceAll(normalized, "\n", "\r\n")
	}
	return normalized
}

// withLF turns every CRLF of the content into LF.
func withLF(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

// IsDirty reports whether content differs from the content the editor loaded, identified by its digest.
func IsDirty(content string, base string) bool {
	return ContentDigest(content) != base
}

// Rows is the height of the textarea: one row per line of the content plus one, within the bounds.
func (e FileEditor) Rows() int {
	lines := strings.Count(e.Content, "\n")
	if e.Content != "" && !strings.HasSuffix(e.Content, "\n") {
		lines++
	}
	return min(max(lines+1, editorMinRows), editorMaxRows)
}

// SlotID is the element that holds the editor of the file in its card. Each instance of the view scopes it.
func (e FileEditor) SlotID() string {
	return FileSlotID(e.Scope, e.Path)
}

// StatusID is the element that shows whether the editor holds unsaved changes.
func (e FileEditor) StatusID() string {
	return e.SlotID() + "-status"
}

// SavePath is the address that saves the file of the editor.
func (e FileEditor) SavePath() string {
	return TaskFilePath(e.TaskID)
}

// StatePath is the address that reports whether the editor holds unsaved changes.
func (e FileEditor) StatePath() string {
	return TaskFilePath(e.TaskID) + "/state"
}

// ClosePath is the address that closes the editor without saving.
func (e FileEditor) ClosePath() string {
	return TaskFilePath(e.TaskID) + "/close"
}

// TaskFilePath is the address of the file editor of a task, without the query of one file.
func TaskFilePath(taskID string) string {
	return "/tasks/" + taskID + "/file"
}

// FileEditorPath is the address that loads the editor of one file in a scope.
func FileEditorPath(taskID string, scope string, path string) string {
	values := url.Values{}
	values.Set("scope", scope)
	values.Set("path", path)
	return TaskFilePath(taskID) + "?" + values.Encode()
}

// FileSlotID is the element of the card of a file that holds its editor. It is the same for every instance of a view
// of the same scope, so the card and the editor it loads agree on it.
func FileSlotID(scope string, path string) string {
	return Scoped(AnchorID(path)+"-editor", scope)
}
