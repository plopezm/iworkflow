package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	engrammodels "iworkflow/internal/modules/engram/domain/models"
	memorieserrors "iworkflow/internal/modules/memories/domain/errors"
	memoriesmodels "iworkflow/internal/modules/memories/domain/models"
	memoriesoutbound "iworkflow/internal/modules/memories/domain/ports/outbound"
	tasksapp "iworkflow/internal/modules/tasks/application"
	taskserrors "iworkflow/internal/modules/tasks/domain/errors"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	workspacesapp "iworkflow/internal/modules/workspaces/application"
)

// engramListReader and engramMemoryReader are the parts of the engram use cases the memories bridge needs.
type engramListReader interface {
	Execute(ctx context.Context, workspace engrammodels.Workspace) (engrammodels.MemoryFeed, error)
}

type engramMemoryReader interface {
	Execute(ctx context.Context, workspace engrammodels.Workspace, id int) (engrammodels.MemoryEntry, error)
}

// engramMemoryProvider is the Engram adapter of the memories facade. It lives in the composition root, so the
// memories module never imports engram: swapping the provider means replacing only this type.
type engramMemoryProvider struct {
	list engramListReader
	read engramMemoryReader
}

var _ memoriesoutbound.MemoryProvider = (*engramMemoryProvider)(nil)

func newEngramMemoryProvider(list engramListReader, read engramMemoryReader) *engramMemoryProvider {
	return &engramMemoryProvider{list: list, read: read}
}

// List maps the Engram feed of the workspace. No matching project reads as empty, an unreachable Engram as unavailable.
func (p *engramMemoryProvider) List(ctx context.Context, workspace memoriesmodels.Workspace) (memoriesmodels.Feed, error) {
	feed, err := p.list.Execute(ctx, engrammodels.NewWorkspace(workspace.Name, workspace.Path))
	if err != nil {
		return memoriesmodels.Feed{}, fmt.Errorf("list engram memories: %w", err)
	}
	state := memoriesState(feed.State)
	if state != memoriesmodels.StateAvailable {
		return memoriesmodels.Feed{State: state}, nil
	}
	memories := make([]memoriesmodels.Memory, 0, len(feed.Memories))
	for _, memory := range feed.Memories {
		memories = append(memories, memoriesMemory(memory))
	}
	return memoriesmodels.Feed{State: memoriesmodels.StateAvailable, Memories: memories}, nil
}

// Read maps one Engram memory of the workspace. A memory of another project is reported as not found.
func (p *engramMemoryProvider) Read(ctx context.Context, workspace memoriesmodels.Workspace, id int) (memoriesmodels.Entry, error) {
	entry, err := p.read.Execute(ctx, engrammodels.NewWorkspace(workspace.Name, workspace.Path), id)
	if errors.Is(err, engramerrors.ErrNotFound) {
		return memoriesmodels.Entry{}, fmt.Errorf("engram memory %d: %w", id, memorieserrors.ErrMemoryNotFound)
	}
	if err != nil {
		return memoriesmodels.Entry{}, fmt.Errorf("read engram memory %d: %w", id, err)
	}
	state := memoriesState(entry.State)
	if state != memoriesmodels.StateAvailable {
		return memoriesmodels.Entry{State: state}, nil
	}
	memory := memoriesMemory(entry.Detail.Memory)
	if !entry.Detail.UpdatedAt.IsZero() {
		memory.UpdatedAt = entry.Detail.UpdatedAt
	}
	return memoriesmodels.Entry{State: memoriesmodels.StateAvailable, Memory: memory, Content: entry.Detail.Content}, nil
}

func memoriesState(state engrammodels.MemoryState) memoriesmodels.State {
	switch state {
	case engrammodels.StateAvailable:
		return memoriesmodels.StateAvailable
	case engrammodels.StateNoMatch:
		return memoriesmodels.StateEmpty
	default:
		return memoriesmodels.StateUnavailable
	}
}

func memoriesMemory(memory engrammodels.Memory) memoriesmodels.Memory {
	return memoriesmodels.Memory{ID: memory.ID, Type: memory.Type, Title: memory.Title, UpdatedAt: memory.LastSeenAt}
}

// memoriesWorkspaceLookup gives the memories module the registered workspaces, through the workspaces use cases.
type memoriesWorkspaceLookup struct {
	get *workspacesapp.GetWorkspace
}

var _ memoriesoutbound.WorkspaceLookup = (*memoriesWorkspaceLookup)(nil)

// Get returns the workspace, or an error wrapping ErrWorkspaceNotFound for an unknown or malformed id.
func (l *memoriesWorkspaceLookup) Get(ctx context.Context, id string) (memoriesmodels.Workspace, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return memoriesmodels.Workspace{}, fmt.Errorf("workspace %q: %w", id, memorieserrors.ErrWorkspaceNotFound)
	}
	workspace, err := findWorkspace(ctx, l.get, parsed)
	if err != nil {
		return memoriesmodels.Workspace{}, err
	}
	if workspace == nil {
		return memoriesmodels.Workspace{}, fmt.Errorf("workspace %q: %w", id, memorieserrors.ErrWorkspaceNotFound)
	}
	return memoriesmodels.Workspace{ID: workspace.ID.String(), Name: workspace.Name, Path: workspace.Path}, nil
}

// memoriesTaskLookup gives the memories module the task whose branch memories are read, through the tasks use cases. A
// task still in the backlog has no branch yet, so it reports as not started.
type memoriesTaskLookup struct {
	get *tasksapp.GetTask
}

var _ memoriesoutbound.TaskLookup = (*memoriesTaskLookup)(nil)

// Get returns the task, or an error wrapping ErrTaskNotFound for an unknown or malformed id.
func (l *memoriesTaskLookup) Get(ctx context.Context, id string) (memoriesmodels.TaskRef, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return memoriesmodels.TaskRef{}, fmt.Errorf("task %q: %w", id, memorieserrors.ErrTaskNotFound)
	}
	task, err := l.get.Execute(ctx, tasksmodels.TaskIDFromUUID(parsed))
	if errors.Is(err, taskserrors.NotFound("")) {
		return memoriesmodels.TaskRef{}, fmt.Errorf("task %q: %w", id, memorieserrors.ErrTaskNotFound)
	}
	if err != nil {
		return memoriesmodels.TaskRef{}, fmt.Errorf("find task: %w", err)
	}
	return memoriesmodels.TaskRef{
		ID:          task.ID.String(),
		WorkspaceID: task.WorkspaceID,
		Title:       task.Title,
		Branch:      task.Branch,
		Started:     task.Status != tasksmodels.StatusBacklog,
		StartedAt:   task.CreatedAt,
	}, nil
}
