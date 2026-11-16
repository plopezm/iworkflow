// Package di is the composition root of the memories module.
package di

import (
	"context"

	"github.com/a-h/templ"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/memories/application"
	"iworkflow/internal/modules/memories/domain/ports/outbound"
	"iworkflow/internal/modules/memories/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/memories/presentation/pages"
)

// Deps are what the module needs. Provider may be nil: the pages then say the memory system is not available.
type Deps struct {
	Workspaces outbound.WorkspaceLookup
	Tasks      outbound.TaskLookup
	Provider   outbound.MemoryProvider
}

// Module exposes the read-only use cases and the dock tabs of the memories module.
type Module struct {
	List     *application.ListWorkspaceMemories
	Read     *application.ReadWorkspaceMemory
	TaskList *application.ListTaskMemories
	handlers *inbound.Handlers
}

// NewModule builds the use cases and the handlers.
func NewModule(deps Deps) *Module {
	list := application.NewListWorkspaceMemories(deps.Workspaces, deps.Provider)
	read := application.NewReadWorkspaceMemory(deps.Workspaces, deps.Provider)
	taskList := application.NewListTaskMemories(deps.Tasks, deps.Workspaces, deps.Provider)
	return &Module{List: list, Read: read, TaskList: taskList, handlers: inbound.NewHandlers(list, read, taskList)}
}

// Tabs returns the dock routes of the module: the memories of a workspace, the memories of a task and one memory.
func (m *Module) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "workspace_memories",
			Pattern: "/workspaces/{id}/memories",
			Icon:    "◈",
			Title:   func(ctx context.Context, _ dock.Params) string { return pages.ListTitle(ctx) },
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.ListTab(ctx, params)
			},
		},
		{
			Name:    "task_memories",
			Pattern: "/tasks/{taskId}/memories",
			Icon:    "◈",
			Title:   func(ctx context.Context, _ dock.Params) string { return pages.ListTitle(ctx) },
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.TaskTab(ctx, params)
			},
		},
		{
			Name:    "workspace_memory",
			Pattern: "/workspaces/{id}/memories/{memoryId}",
			Icon:    "◈",
			Title:   func(ctx context.Context, _ dock.Params) string { return inbound.DetailTitle(ctx) },
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return m.handlers.DetailTab(ctx, params)
			},
		},
	}
}
