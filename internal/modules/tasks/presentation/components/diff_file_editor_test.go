package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

const (
	editTaskID = "7d3c2b1a-0000-4000-8000-000000000001"
	editScope  = "7d3c2b1a-0000-4000-8000-000000000002"
)

// editableView is the diff of a task that still has its worktree, so its files can be edited.
func editableView(scope string, canEdit bool) state.DiffView {
	return state.DiffView{
		Scope:   scope,
		TaskID:  editTaskID,
		CanEdit: canEdit,
		Layout:  state.DiffLayout{Mode: state.ViewSplit},
	}
}

func TestFileCardOffersEditForEditableFiles(t *testing.T) {
	t.Parallel()
	file := patchFile(t, modifyPatch)

	html := renderIn(t, i18n.En, components.DiffFileCard(file, editableView(editScope, true), true))

	assert.Contains(t, html, `hx-get="/tasks/`+editTaskID+`/file?`)
	assert.Contains(t, html, "path=src%2Fa.rs")
	assert.Contains(t, html, "scope="+editScope)
	assert.Contains(t, html, ">Edit<")
	assert.Contains(t, html, `id="`+state.FileSlotID(editScope, "src/a.rs")+`"`)
}

func TestFileCardHasNoEditWhereTheFileCannotBeEdited(t *testing.T) {
	t.Parallel()
	file := patchFile(t, modifyPatch)
	binary := state.FileDiff{Path: "logo.png", Status: state.FileStatusModified, Binary: true}
	deleted := state.FileDiff{Path: "old.rs", Status: state.FileStatusDeleted}

	assert.NotContains(t, renderIn(t, i18n.En, components.DiffFileCard(file, editableView(editScope, false), true)), ">Edit<",
		"a task without a worktree has nothing to edit")
	assert.NotContains(t, renderIn(t, i18n.En, components.DiffFileCard(binary, editableView(editScope, true), true)), ">Edit<")
	assert.NotContains(t, renderIn(t, i18n.En, components.DiffFileCard(deleted, editableView(editScope, true), true)), ">Edit<")
}

func TestEditorHoldsTheContentAndTheDigestItLoaded(t *testing.T) {
	t.Parallel()
	editor := state.FileEditor{Scope: editScope, TaskID: editTaskID, Path: "src/a.rs", Content: "a\nb\n", Digest: "abc"}

	html := renderIn(t, i18n.En, components.TaskFileEditor(editor))

	assert.Contains(t, html, `hx-post="/tasks/`+editTaskID+`/file"`)
	assert.Contains(t, html, `hx-post="/tasks/`+editTaskID+`/file/state"`)
	assert.Contains(t, html, `name="base" value="abc"`)
	assert.Contains(t, html, `rows="8"`)
	assert.Contains(t, html, ">a\nb\n</textarea>")
	assert.Contains(t, html, ">Save<")
}

func TestEditorKeepsALeadingLineBreakOfItsContent(t *testing.T) {
	t.Parallel()
	editor := state.FileEditor{Scope: editScope, TaskID: editTaskID, Path: "a.txt", Content: "\nx"}

	html := renderIn(t, i18n.En, components.TaskFileEditor(editor))

	assert.Contains(t, html, ">\n\nx</textarea>", "the parser drops the first break, so the editor writes two")
}

func TestEditorShowsItsLastSaveOutcome(t *testing.T) {
	t.Parallel()
	editor := state.FileEditor{Scope: editScope, TaskID: editTaskID, Path: "a.txt", Content: "x", Unchanged: true}

	html := renderIn(t, i18n.Es, components.TaskFileEditor(editor))
	assert.Contains(t, html, "Sin cambios: el fichero ya tenía ese contenido, no se creó ningún commit.")

	editor = state.FileEditor{Scope: editScope, TaskID: editTaskID, Path: "a.txt", Content: "x", Error: "No se pudo guardar el fichero."}
	assert.Contains(t, renderIn(t, i18n.Es, components.TaskFileEditor(editor)), "No se pudo guardar el fichero.")
}

func TestStatusNamesUnsavedChangesOnlyWhenDirty(t *testing.T) {
	t.Parallel()

	assert.Contains(t, renderIn(t, i18n.En, components.TaskFileStatus(true)), "Unsaved changes")
	assert.NotContains(t, renderIn(t, i18n.En, components.TaskFileStatus(false)), "Unsaved changes")
}

func TestDiffViewReloadsWhenAFileIsSaved(t *testing.T) {
	t.Parallel()
	view := editableView(editScope, true)
	view.Diff.Patch = modifyPatch

	html := renderIn(t, i18n.En, components.DiffView(view))

	assert.Contains(t, html, `hx-trigger="iwf-diff-saved from:body"`)
	assert.Contains(t, html, `hx-get="/tasks/`+editTaskID+`/diff/view?`)
}

// cardView is the diff view a card is drawn in: the scope and the layout mode, without any editing.
func cardView(scope string, mode state.ViewMode) state.DiffView {
	return state.DiffView{Scope: scope, Layout: state.DiffLayout{Mode: mode}}
}

// The editor slot of an editable card carries hx-preserve, so an editor that is open keeps its text when the diff view
// reloads after another file is saved.
func TestFileSlotIsPreservedAcrossADiffReload(t *testing.T) {
	t.Parallel()
	file := patchFile(t, modifyPatch)

	html := renderIn(t, i18n.En, components.DiffFileCard(file, editableView(editScope, true), true))

	assert.Contains(t, html, `id="`+state.FileSlotID(editScope, "src/a.rs")+`" hx-preserve="true"`)
}
