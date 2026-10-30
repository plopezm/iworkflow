package state

import (
	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/linediff"
	"iworkflow/internal/modules/project_tree/domain/models"
)

// FileView is the editor of one project file: the draft under edit, the content on disk, and the line
// marks of the draft against git HEAD.
type FileView struct {
	Scope models.ProjectScope
	Path  string
	// Draft is the text in the editor.
	Draft string
	// Loaded is the content on disk: the draft is dirty when they differ.
	Loaded string
	// Marks holds one mark per line of Draft; empty while git has no base for the file.
	Marks []coreui.LineMark
	// Error is the message of a failed save. The draft stays in the editor.
	Error string
	// LoadError replaces the editor when the file could not be read.
	LoadError string
}

// Name is the file name shown in the tab and in the editor.
func (v FileView) Name() string {
	return FileName(v.Path)
}

// ScopeWire is the scope in its wire form, for URLs and hidden fields.
func (v FileView) ScopeWire() string {
	return v.Scope.Wire()
}

// Key identifies the editor of this file in the DOM.
func (v FileView) Key() string {
	return ID("pt-file", v.ScopeWire(), v.Path)
}

// Dirty reports whether the draft differs from the content on disk.
func (v FileView) Dirty() bool {
	return IsDirty(v.Draft, v.Loaded)
}

// IsDirty reports whether the editor holds changes that are not on disk yet (pure).
func IsDirty(draft string, loaded string) bool {
	return draft != loaded
}

// MarksFor marks the lines of draft against base. A nil base (the file is not in git HEAD) has no
// marks (pure).
func MarksFor(base *string, draft string) []coreui.LineMark {
	if base == nil {
		return nil
	}
	marks := linediff.LineMarks(*base, draft)
	result := make([]coreui.LineMark, 0, len(marks))
	for _, mark := range marks {
		result = append(result, lineMark(mark))
	}
	return result
}

// lineMark maps a diff mark to the mark of the editor gutter (pure).
func lineMark(mark linediff.Mark) coreui.LineMark {
	switch mark {
	case linediff.Added:
		return coreui.LineAdded
	case linediff.Modified:
		return coreui.LineModified
	case linediff.Deleted:
		return coreui.LineDeleted
	default:
		return ""
	}
}

// MarkAt returns the mark of line i, or "" (unmarked) when there are fewer marks than lines (pure).
func MarkAt(marks []coreui.LineMark, i int) coreui.LineMark {
	if i < len(marks) {
		return marks[i]
	}
	return ""
}
