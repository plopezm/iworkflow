//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// branchSlot is the id of the base branch select of the create dialog; a workspace change replaces it.
const branchSlot = "board-branch-slot"

// branches serves the base branch select of the create dialog for the workspace the user picked.
func (h *BoardHandlers) branches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspace_id")
	workspaces, err := h.workspaceOptions(ctx)
	if err != nil {
		web.Render(w, r, http.StatusOK, components.BranchSlotFailed(branchSlot, err.Error()))
		return
	}
	options, err := h.branchesOf(ctx, workspaceID)
	if err != nil {
		web.Render(w, r, http.StatusOK, components.BranchSlotFailed(branchSlot, err.Error()))
		return
	}
	choices := state.BranchChoices(options, "")
	web.Render(w, r, http.StatusOK, components.BranchSelect(branchSlot, choices, "", defaultBranchOf(workspaces, workspaceID)))
}

// startTask starts a backlog task from its card. The list refreshes either way, because a failed start may have
// put the task back in backlog with its error; the failure is shown in the notice slot.
func (h *BoardHandlers) startTask(w http.ResponseWriter, r *http.Request) {
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, components.BoardNotice(err.Error()))
		return
	}
	_, startErr := h.uc.Start.Start(r.Context(), id)
	web.Trigger(w, boardRefreshEvent)
	web.Render(w, r, http.StatusOK, components.BoardNotice(errorText(startErr)))
}

// deleteTask deletes a backlog task from its card. The confirmation is asked by the card before the request.
func (h *BoardHandlers) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, components.BoardNotice(err.Error()))
		return
	}
	deleteErr := h.uc.Delete.Execute(r.Context(), id)
	web.Trigger(w, boardRefreshEvent)
	web.Render(w, r, http.StatusOK, components.BoardNotice(errorText(deleteErr)))
}

// editTab is the edit tab of a backlog task. A task that left backlog has no edit page any more.
func (h *BoardHandlers) editTab(ctx context.Context, rawID string) templ.Component {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	form, editable, err := h.loadEdit(ctx, id)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	if !editable {
		return components.NotEditable()
	}
	return pages.TaskEditPage(form)
}

// updateTask saves the edit page of a backlog task. The form is validated before anything is changed, and a
// rejected form keeps the page and its typed values, with the error shown in place of the result.
func (h *BoardHandlers) updateTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, components.TaskEditResult(err.Error()))
		return
	}
	form, err := dto.ParseTaskForm(w, r)
	if err != nil {
		web.Render(w, r, http.StatusOK, components.TaskEditResult(err.Error()))
		return
	}
	current, editable, err := h.loadEdit(ctx, id)
	if err != nil {
		web.Render(w, r, http.StatusOK, components.TaskEditResult(err.Error()))
		return
	}
	if !editable {
		web.Render(w, r, http.StatusOK, components.NotEditable())
		return
	}
	input, err := form.ToUpdate(keptImages(current.Existing, form.RemoveIDs))
	if err != nil {
		web.Render(w, r, http.StatusOK, components.TaskEditResult(err.Error()))
		return
	}
	saved, err := h.uc.Update.Execute(ctx, id, input)
	if err != nil {
		web.Render(w, r, http.StatusOK, components.TaskEditResult(err.Error()))
		return
	}
	web.Trigger(w, boardRefreshEvent)
	web.Render(w, r, http.StatusOK, components.TaskSaved(state.BoardPath(saved.WorkspaceID)))
}

// loadEdit reads what the edit page of a task shows. editable is false for a task that left backlog.
func (h *BoardHandlers) loadEdit(ctx context.Context, id models.TaskID) (state.EditForm, bool, error) {
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil {
		return state.EditForm{}, false, fmt.Errorf("read the task: %w", err)
	}
	if !task.Status.CanEdit() {
		return state.EditForm{}, false, nil
	}
	stored, err := h.uc.ListAttachments.Execute(ctx, id)
	if err != nil {
		return state.EditForm{}, false, fmt.Errorf("list the images: %w", err)
	}
	existing := make([]dto.TaskAttachmentDTO, 0, len(stored))
	for _, attachment := range stored {
		existing = append(existing, dto.TaskAttachmentFromDomain(id.String(), attachment))
	}
	workspaces, err := h.workspaceOptions(ctx)
	if err != nil {
		return state.EditForm{}, false, err
	}
	// A failed branch listing does not stop the page: the task keeps its base branch, and the failure is shown.
	branches, branchErr := h.branchesOf(ctx, task.WorkspaceID)
	view := dto.TaskFromDomain(task, h.now())
	form := state.EditForm{
		TaskID:        id.String(),
		WorkspaceID:   task.WorkspaceID,
		Title:         view.Title,
		Prompt:        view.Prompt,
		Priority:      view.Priority,
		BaseBranch:    view.BaseBranch,
		DefaultBranch: defaultBranchOf(workspaces, task.WorkspaceID),
		Branches:      branches,
		MemoryLimit:   limitText(view.MemoryLimitMB),
		CPULimit:      limitText(view.CPUPercent),
		ShowAdvanced:  view.MemoryLimitMB != nil || view.CPUPercent != nil,
		Existing:      existing,
		BoardPath:     state.BoardPath(task.WorkspaceID),
	}
	if branchErr != nil {
		form.Error = branchErr.Error()
	}
	return form, true, nil
}

// keptImages counts the stored images that a save keeps: the ones not marked for removal.
func keptImages(existing []dto.TaskAttachmentDTO, removed []string) int {
	kept := 0
	for _, attachment := range existing {
		if !contains(removed, attachment.ID) {
			kept++
		}
	}
	return kept
}

// contains reports whether value is one of list.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// limitText is the form text of an optional limit: empty when the task uses the default.
func limitText(limit *uint32) string {
	if limit == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*limit), 10)
}

// errorText is the message of an error, or empty when there is none.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
