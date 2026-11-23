//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"strconv"

	"github.com/a-h/templ"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	memerrors "iworkflow/internal/modules/memories/domain/errors"
	"iworkflow/internal/modules/memories/domain/models"
	"iworkflow/internal/modules/memories/presentation/pages"
	"iworkflow/internal/modules/memories/presentation/state"
)

// listUseCase and readUseCase are the parts of the use cases the tabs need.
type listUseCase interface {
	Execute(ctx context.Context, workspaceID string) (models.Workspace, models.Feed, error)
}

type readUseCase interface {
	Execute(ctx context.Context, workspaceID string, memoryID int) (models.Workspace, models.Entry, error)
}

type taskListUseCase interface {
	Execute(ctx context.Context, taskID string) (models.TaskRef, models.Workspace, models.TaskFeed, error)
}

// Handlers renders the read-only memory tabs. It has no mutating route.
type Handlers struct {
	list  listUseCase
	read  readUseCase
	tasks taskListUseCase
}

// NewHandlers builds the handlers.
func NewHandlers(list listUseCase, read readUseCase, tasks taskListUseCase) *Handlers {
	return &Handlers{list: list, read: read, tasks: tasks}
}

const dateLayout = "2006-01-02 15:04"

// ListTab renders the memories of one workspace.
func (h *Handlers) ListTab(ctx context.Context, params dock.Params) templ.Component {
	id := params["id"]
	workspace, feed, err := h.list.Execute(ctx, id)
	if err != nil {
		return shared.ErrorText(errorText(err))
	}
	page := state.ListPage{
		WorkspaceName:  workspace.Name,
		WorkspaceColor: workspace.Color,
		BoardHref:      "/board/" + workspace.ID,
		Availability:   availability(feed.State, len(feed.Memories)),
	}
	for _, memory := range feed.Memories {
		page.Rows = append(page.Rows, state.Row{
			Type:  memory.Type,
			Title: memory.Title,
			Date:  formatDate(memory),
			Href:  state.DetailPathFor(workspace.ID, memory.ID),
		})
	}
	return pages.MemoriesPage(page)
}

// DetailTab renders one memory of one workspace.
func (h *Handlers) DetailTab(ctx context.Context, params dock.Params) templ.Component {
	memoryID, err := strconv.Atoi(params["memoryId"])
	if err != nil || memoryID <= 0 {
		return shared.ErrorText(errorText(memerrors.ErrMemoryNotFound))
	}
	workspace, entry, err := h.read.Execute(ctx, params["id"], memoryID)
	if err != nil {
		return shared.ErrorText(errorText(err))
	}
	if entry.State == models.StateEmpty {
		// The workspace has no memory system project, so there is no memory to read.
		return shared.ErrorText(errorText(memerrors.ErrMemoryNotFound))
	}
	return pages.MemoryPage(state.DetailPage{
		WorkspaceName:  workspace.Name,
		WorkspaceColor: workspace.Color,
		ListHref:       state.ListPathFor(workspace.ID),
		Availability:   availability(entry.State, 1),
		Type:           entry.Memory.Type,
		Title:          entry.Memory.Title,
		Date:           formatDate(entry.Memory),
		Content:        entry.Content,
	})
}

// TaskTab renders the memories of one task: the ones written since its branch started, and the earlier ones.
func (h *Handlers) TaskTab(ctx context.Context, params dock.Params) templ.Component {
	task, workspace, feed, err := h.tasks.Execute(ctx, params["taskId"])
	if err != nil {
		return shared.ErrorText(errorText(err))
	}
	return pages.TaskMemoriesPage(state.TaskPage{
		TaskTitle:     task.Title,
		Branch:        task.Branch,
		WorkspaceName: workspace.Name,
		TaskHref:      "/tasks/" + task.ID,
		ListHref:      state.ListPathFor(workspace.ID),
		Availability:  availability(feed.State, len(feed.Branch)+len(feed.Earlier)),
		Started:       task.Started,
		BranchRows:    rows(workspace.ID, feed.Branch),
		EarlierRows:   rows(workspace.ID, feed.Earlier),
	})
}

func rows(workspaceID string, memories []models.Memory) []state.Row {
	out := make([]state.Row, 0, len(memories))
	for _, memory := range memories {
		out = append(out, state.Row{
			Type:  memory.Type,
			Title: memory.Title,
			Date:  formatDate(memory),
			Href:  state.DetailPathFor(workspaceID, memory.ID),
		})
	}
	return out
}

// DetailTitle is the tab title of one memory.
func DetailTitle(ctx context.Context) string { return pages.ListTitle(ctx) }

func availability(s models.State, rows int) state.Availability {
	switch {
	case s == models.StateUnavailable:
		return state.Unavailable
	case s == models.StateEmpty || rows == 0:
		return state.Empty
	default:
		return state.Ready
	}
}

func formatDate(memory models.Memory) string {
	if memory.UpdatedAt.IsZero() {
		return ""
	}
	return memory.UpdatedAt.Local().Format(dateLayout)
}

// errorText maps a domain error to the text of the tab. Unexpected failures never leak their detail.
func errorText(err error) string {
	switch {
	case errors.Is(err, memerrors.ErrWorkspaceNotFound):
		return "workspace not found"
	case errors.Is(err, memerrors.ErrTaskNotFound):
		return "task not found"
	case errors.Is(err, memerrors.ErrMemoryNotFound):
		return "memory not found"
	default:
		return "the memories could not be read"
	}
}
