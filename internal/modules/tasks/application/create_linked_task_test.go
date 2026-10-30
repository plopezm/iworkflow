package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// fakeCreator records the input it was given and returns a task with a fixed identifier.
type fakeCreator struct {
	id    models.TaskID
	input models.NewTask
	err   error
}

func (f *fakeCreator) Execute(_ context.Context, input models.NewTask) (models.CreatedTask, error) {
	f.input = input
	if f.err != nil {
		return models.CreatedTask{}, f.err
	}
	return models.CreatedTask{Task: models.Task{ID: f.id, WorkspaceID: input.WorkspaceID}}, nil
}

// fakeEdges records every edge it is asked to add and fails the successors listed in failFor.
type fakeEdges struct {
	edges   [][2]models.TaskID
	failFor map[models.TaskID]bool
}

func (f *fakeEdges) Execute(_ context.Context, predecessor models.TaskID, successor models.TaskID) (models.TaskDependency, error) {
	f.edges = append(f.edges, [2]models.TaskID{predecessor, successor})
	if f.failFor[predecessor] {
		return models.TaskDependency{}, errors.New("cycle")
	}
	return models.TaskDependency{Predecessor: predecessor, Successor: successor}, nil
}

// fakeGraph returns a fixed graph, or fails.
type fakeGraph struct {
	graph models.TaskGraph
	err   error
}

func (f fakeGraph) Execute(context.Context, string) (models.TaskGraph, error) {
	return f.graph, f.err
}

func mustTaskID(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.ParseTaskID(uuid.NewString())
	require.NoError(t, err)
	return id
}

func TestCreateLinkedTaskWithoutALinkIsAPlainCreate(t *testing.T) {
	t.Parallel()
	creator := &fakeCreator{id: mustTaskID(t)}
	edges := &fakeEdges{}
	useCase := NewCreateLinkedTask(creator, edges, fakeGraph{})

	result, err := useCase.Execute(context.Background(), models.NewTask{WorkspaceID: "w", StartNow: true}, nil)

	require.NoError(t, err)
	assert.True(t, creator.input.StartNow, "a plain create keeps the start request")
	assert.Empty(t, edges.edges)
	assert.Empty(t, result.LinkErrors)
}

func TestCreateLinkedTaskNextDependsOnTheAnchorAndNeverStartsOnCreation(t *testing.T) {
	t.Parallel()
	created := mustTaskID(t)
	anchor := mustTaskID(t)
	creator := &fakeCreator{id: created}
	edges := &fakeEdges{}
	useCase := NewCreateLinkedTask(creator, edges, fakeGraph{})

	result, err := useCase.Execute(context.Background(), models.NewTask{WorkspaceID: "w", StartNow: true}, &TaskLink{Kind: LinkNext, Anchor: anchor})

	require.NoError(t, err)
	assert.False(t, creator.input.StartNow, "a linked task waits for its predecessors")
	assert.Equal(t, [][2]models.TaskID{{anchor, created}}, edges.edges)
	assert.Empty(t, result.LinkErrors)
}

func TestCreateLinkedTaskParallelTakesThePredecessorsOfTheAnchor(t *testing.T) {
	t.Parallel()
	created := mustTaskID(t)
	anchor := mustTaskID(t)
	firstPredecessor := mustTaskID(t)
	secondPredecessor := mustTaskID(t)
	unrelated := mustTaskID(t)
	graph := fakeGraph{graph: models.TaskGraph{Dependencies: []models.TaskDependency{
		{Predecessor: firstPredecessor, Successor: anchor},
		{Predecessor: secondPredecessor, Successor: anchor},
		{Predecessor: unrelated, Successor: mustTaskID(t)},
	}}}
	edges := &fakeEdges{}
	useCase := NewCreateLinkedTask(&fakeCreator{id: created}, edges, graph)

	_, err := useCase.Execute(context.Background(), models.NewTask{WorkspaceID: "w"}, &TaskLink{Kind: LinkParallel, Anchor: anchor})

	require.NoError(t, err)
	assert.Equal(t, [][2]models.TaskID{{firstPredecessor, created}, {secondPredecessor, created}}, edges.edges)
}

func TestCreateLinkedTaskKeepsTheTaskWhenAnEdgeFails(t *testing.T) {
	t.Parallel()
	created := mustTaskID(t)
	anchor := mustTaskID(t)
	edges := &fakeEdges{failFor: map[models.TaskID]bool{anchor: true}}
	useCase := NewCreateLinkedTask(&fakeCreator{id: created}, edges, fakeGraph{})

	result, err := useCase.Execute(context.Background(), models.NewTask{WorkspaceID: "w"}, &TaskLink{Kind: LinkNext, Anchor: anchor})

	require.NoError(t, err, "the task exists, so the create itself succeeded")
	assert.Equal(t, created, result.Created.Task.ID)
	require.Len(t, result.LinkErrors, 1)
	assert.EqualError(t, result.LinkErrors[0], "cycle")
}

func TestCreateLinkedTaskReportsAGraphReadFailureAsALinkError(t *testing.T) {
	t.Parallel()
	edges := &fakeEdges{}
	useCase := NewCreateLinkedTask(&fakeCreator{id: mustTaskID(t)}, edges, fakeGraph{err: errors.New("graph down")})

	result, err := useCase.Execute(context.Background(), models.NewTask{WorkspaceID: "w"}, &TaskLink{Kind: LinkParallel, Anchor: mustTaskID(t)})

	require.NoError(t, err)
	assert.Empty(t, edges.edges)
	require.Len(t, result.LinkErrors, 1)
	assert.ErrorContains(t, result.LinkErrors[0], "graph down")
}

func TestCreateLinkedTaskCreatesNothingLinkedWhenTheCreateFails(t *testing.T) {
	t.Parallel()
	edges := &fakeEdges{}
	useCase := NewCreateLinkedTask(&fakeCreator{err: errors.New("prompt is required")}, edges, fakeGraph{})

	_, err := useCase.Execute(context.Background(), models.NewTask{WorkspaceID: "w"}, &TaskLink{Kind: LinkNext, Anchor: mustTaskID(t)})

	require.EqualError(t, err, "prompt is required")
	assert.Empty(t, edges.edges)
}

func TestCreateLinkedTaskRejectsAnUnknownLinkKind(t *testing.T) {
	t.Parallel()
	edges := &fakeEdges{}
	useCase := NewCreateLinkedTask(&fakeCreator{id: mustTaskID(t)}, edges, fakeGraph{})

	result, err := useCase.Execute(context.Background(), models.NewTask{WorkspaceID: "w"}, &TaskLink{Kind: "sideways", Anchor: mustTaskID(t)})

	require.NoError(t, err)
	assert.Empty(t, edges.edges)
	require.Len(t, result.LinkErrors, 1)
	assert.ErrorContains(t, result.LinkErrors[0], "unknown link kind")
}
