//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package inbound

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/presentation/state"
)

// rootView lists the root of a scope. A failure is returned with a view that shows its message, so the
// caller can render the listing and still choose the status.
func (h *Handlers) rootView(ctx context.Context, scope models.ProjectScope) (state.TreeView, error) {
	directory, err := h.uc.ListDirectory.Execute(ctx, scope, "")
	if err != nil {
		return state.TreeErrorView(scope, err.Error()), err
	}
	return state.NewRootView(scope, directory.WorkspaceName, directory.Contents.Entries, h.statusesOf(ctx, scope)), nil
}

// childrenView lists one folder for an expansion. A failure works as in rootView.
func (h *Handlers) childrenView(ctx context.Context, scope models.ProjectScope, folder string) (state.TreeView, error) {
	directory, err := h.uc.ListDirectory.Execute(ctx, scope, folder)
	if err != nil {
		return state.TreeErrorView(scope, err.Error()), err
	}
	return state.NewChildrenView(scope, folder, directory.Contents.Entries, h.statusesOf(ctx, scope)), nil
}

// fileView reads the file on disk and builds its editor. draft is the text in the editor; nil means the
// content on disk. The marks compare the draft with git HEAD, and without a base there are none.
func (h *Handlers) fileView(ctx context.Context, scope models.ProjectScope, path string, draft *string) (state.FileView, error) {
	file, err := h.uc.ReadFile.Execute(ctx, scope, path)
	if err != nil {
		return state.FileView{}, err
	}
	text := file.Content
	if draft != nil {
		text = *draft
	}
	base, baseErr := h.uc.FileBase.Execute(ctx, scope, path)
	if baseErr != nil {
		base = nil
	}
	return state.FileView{
		Scope:  scope,
		Path:   path,
		Draft:  text,
		Loaded: file.Content,
		Marks:  state.MarksFor(base, text),
	}, nil
}

// gitView reads the changes of the scope. A failure becomes the load error of the panel.
func (h *Handlers) gitView(ctx context.Context, scope models.ProjectScope) state.GitView {
	changes, err := h.uc.GitChanges.Execute(ctx, scope)
	if err != nil {
		return state.GitView{Scope: scope, LoadError: err.Error()}
	}
	return state.GitView{Scope: scope, Groups: state.GroupChanges(changes)}
}
