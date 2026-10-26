package state

import (
	"context"
	"fmt"
	"strings"

	"iworkflow/internal/core/i18n"
)

// View helpers of the task diff. Port of the view part of src/modules/tasks/presentation/state/diff_view_state.rs:
// the layout mode, the DOM ids of the cards and the edit rule. The screen state itself lives in the URL of the
// diff view, so a reload keeps it (see the diff handlers).

// Texts of the diff view controls, copied from the Rust Msg constants they replace.
var (
	msgSplit   = i18n.Msg{En: "Split", Es: "Dividido", Zh: "分栏", De: "Geteilt", Fr: "Côte à côte"}
	msgUnified = i18n.Msg{En: "Unified", Es: "Unificado", Zh: "统一", De: "Vereint", Fr: "Unifié"}
)

// anchorPrefix starts the DOM id of every file card; the tree links point at it.
const anchorPrefix = "diff-file-"

// ViewMode is how the rows of a file are laid out.
type ViewMode string

// The layouts of a file diff.
const (
	ViewSplit   ViewMode = "split"
	ViewUnified ViewMode = "unified"
)

// ParseViewMode reads the layout of a query value. Anything but "unified" is the split layout, the default.
func ParseViewMode(raw string) ViewMode {
	if raw == string(ViewUnified) {
		return ViewUnified
	}
	return ViewSplit
}

// Toggled is the other layout.
func (m ViewMode) Toggled() ViewMode {
	if m == ViewSplit {
		return ViewUnified
	}
	return ViewSplit
}

// Label is the name of the layout in the language of the request.
func (m ViewMode) Label(ctx context.Context) string {
	if m == ViewUnified {
		return msgUnified.T(ctx)
	}
	return msgSplit.T(ctx)
}

// AnchorID is the DOM id of the card of path. Every byte that is not alphanumeric is escaped, so distinct paths
// never share an id and the id is safe in a fragment.
func AnchorID(path string) string {
	var id strings.Builder
	id.WriteString(anchorPrefix)
	for index := range len(path) {
		char := path[index]
		if isAlphanumeric(char) {
			id.WriteByte(char)
			continue
		}
		fmt.Fprintf(&id, "_%02x", char)
	}
	return id.String()
}

func isAlphanumeric(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')
}

// CanEditFile says whether a file card offers the editor. Deleted and binary files cannot be edited, and nothing
// can be edited when the task has no worktree.
func CanEditFile(file FileDiff, canEditFiles bool) bool {
	return canEditFiles && !file.Binary && file.Status != FileStatusDeleted
}
