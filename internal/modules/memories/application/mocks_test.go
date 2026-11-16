package application

import (
	"context"

	memerrors "iworkflow/internal/modules/memories/domain/errors"
	"iworkflow/internal/modules/memories/domain/models"
)

type fakeWorkspaces struct{ known map[string]models.Workspace }

func (f fakeWorkspaces) Get(_ context.Context, id string) (models.Workspace, error) {
	workspace, ok := f.known[id]
	if !ok {
		return models.Workspace{}, memerrors.ErrWorkspaceNotFound
	}
	return workspace, nil
}

type fakeProvider struct {
	feed  models.Feed
	entry models.Entry
	err   error
}

func (f fakeProvider) List(context.Context, models.Workspace) (models.Feed, error) {
	return f.feed, f.err
}

func (f fakeProvider) Read(context.Context, models.Workspace, int) (models.Entry, error) {
	return f.entry, f.err
}

type fakeTasks struct{ known map[string]models.TaskRef }

func (f fakeTasks) Get(_ context.Context, id string) (models.TaskRef, error) {
	task, ok := f.known[id]
	if !ok {
		return models.TaskRef{}, memerrors.ErrTaskNotFound
	}
	return task, nil
}
