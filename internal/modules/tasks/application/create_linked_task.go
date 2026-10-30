//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/tasks/domain/models"
)

// LinkKind says how a task created from the board joins the dependency graph.
type LinkKind string

// The link kinds. Next depends on the anchor; Parallel takes the anchor's predecessors, so it waits for the
// same tasks as the anchor (the Rust "Next" and "Parallel" choices).
const (
	LinkNext     LinkKind = "next"
	LinkParallel LinkKind = "parallel"
)

// TaskLink is the dependency a new task is created with: its kind and the task it is linked to.
type TaskLink struct {
	Kind   LinkKind
	Anchor models.TaskID
}

// taskCreator is the part of CreateTask the linked create needs (consumer-side port).
type taskCreator interface {
	Execute(ctx context.Context, input models.NewTask) (models.CreatedTask, error)
}

// dependencyAdder is the part of AddTaskDependency the linked create needs (consumer-side port).
type dependencyAdder interface {
	Execute(ctx context.Context, predecessor models.TaskID, successor models.TaskID) (models.TaskDependency, error)
}

// graphReader is the part of GetTaskGraph the linked create needs (consumer-side port).
type graphReader interface {
	Execute(ctx context.Context, workspaceID string) (models.TaskGraph, error)
}

// CreateLinkedTask creates a task from the board and links it to the graph. A linked task never starts on
// creation: it waits for its predecessors. Each edge that fails is reported in LinkErrors, and the task stays
// created, as the Rust board does.
type CreateLinkedTask struct {
	create        taskCreator
	addDependency dependencyAdder
	graph         graphReader
}

// NewCreateLinkedTask builds the use case over the create, dependency and graph use cases.
func NewCreateLinkedTask(create taskCreator, addDependency dependencyAdder, graph graphReader) *CreateLinkedTask {
	return &CreateLinkedTask{create: create, addDependency: addDependency, graph: graph}
}

// LinkedTask is the result of a linked create: the created task and the edges that could not be added.
type LinkedTask struct {
	Created    models.CreatedTask
	LinkErrors []error
}

// Execute creates the task and, when link is set, adds one edge per predecessor after the creation. A nil
// link is a plain create.
func (u *CreateLinkedTask) Execute(ctx context.Context, input models.NewTask, link *TaskLink) (LinkedTask, error) {
	if link == nil {
		created, err := u.create.Execute(ctx, input)
		return LinkedTask{Created: created}, err
	}
	input.StartNow = false
	created, err := u.create.Execute(ctx, input)
	if err != nil {
		return LinkedTask{}, err
	}
	predecessors, err := u.predecessors(ctx, input.WorkspaceID, *link)
	if err != nil {
		return LinkedTask{Created: created, LinkErrors: []error{err}}, nil
	}
	linkErrors := make([]error, 0)
	for _, predecessor := range predecessors {
		if _, addErr := u.addDependency.Execute(ctx, predecessor, created.Task.ID); addErr != nil {
			linkErrors = append(linkErrors, addErr)
		}
	}
	return LinkedTask{Created: created, LinkErrors: linkErrors}, nil
}

// predecessors lists the tasks the new one must wait for, according to the link kind.
func (u *CreateLinkedTask) predecessors(ctx context.Context, workspaceID string, link TaskLink) ([]models.TaskID, error) {
	switch link.Kind {
	case LinkNext:
		return []models.TaskID{link.Anchor}, nil
	case LinkParallel:
		graph, err := u.graph.Execute(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("read the graph: %w", err)
		}
		var found []models.TaskID
		for _, dependency := range graph.Dependencies {
			if dependency.Successor == link.Anchor {
				found = append(found, dependency.Predecessor)
			}
		}
		return found, nil
	default:
		return nil, fmt.Errorf("unknown link kind %q", link.Kind)
	}
}
