package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// HookListItem is a library hook with where it is assigned and its latest failed run, nil when it never failed.
type HookListItem struct {
	Hook        models.Hook
	Assignments []models.Assignment
	LastFailure *models.HookRun
}

// ListHooks returns the library for the Hooks screen.
type ListHooks struct {
	hooks       outbound.HookRepository
	assignments outbound.AssignmentRepository
	runs        outbound.RunRepository
}

// NewListHooks builds the use case.
func NewListHooks(hooks outbound.HookRepository, assignments outbound.AssignmentRepository, runs outbound.RunRepository) *ListHooks {
	return &ListHooks{hooks: hooks, assignments: assignments, runs: runs}
}

// Execute returns every hook of the library ordered by name.
func (u *ListHooks) Execute(ctx context.Context) ([]HookListItem, error) {
	hooks, err := u.hooks.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list hooks: %w", err)
	}
	failures, err := u.runs.LastFailures(ctx)
	if err != nil {
		return nil, fmt.Errorf("list last failures: %w", err)
	}
	items := make([]HookListItem, 0, len(hooks))
	for _, hook := range hooks {
		assignments, err := u.assignments.ListByHook(ctx, hook.ID())
		if err != nil {
			return nil, fmt.Errorf("list assignments of hook %s: %w", hook.ID(), err)
		}
		item := HookListItem{Hook: hook, Assignments: assignments}
		if failure, ok := failures[hook.ID()]; ok {
			item.LastFailure = &failure
		}
		items = append(items, item)
	}
	return items, nil
}
