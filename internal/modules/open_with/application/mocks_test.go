package application_test

import (
	"context"

	"iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
)

type fakeRepository struct {
	items   []models.Launcher
	saveErr error
}

func (f *fakeRepository) List(context.Context) ([]models.Launcher, error) { return f.items, nil }

func (f *fakeRepository) Get(_ context.Context, id string) (models.Launcher, error) {
	for _, item := range f.items {
		if item.ID() == id {
			return item, nil
		}
	}
	return models.Launcher{}, errors.ErrLauncherNotFound
}

func (f *fakeRepository) Save(_ context.Context, launcher models.Launcher) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.items = append(f.items, launcher)
	return nil
}

func (f *fakeRepository) Delete(_ context.Context, id string) error {
	for i, item := range f.items {
		if item.ID() == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return errors.ErrLauncherNotFound
}

type fakeWorktrees struct {
	path string
	err  error
}

func (f fakeWorktrees) Path(context.Context, string) (string, error) { return f.path, f.err }

type fakeStarter struct {
	specs []models.LaunchSpec
	err   error
}

func (f *fakeStarter) Start(_ context.Context, spec models.LaunchSpec) error {
	if f.err != nil {
		return f.err
	}
	f.specs = append(f.specs, spec)
	return nil
}

type fixedIDs struct{}

func (fixedIDs) NewID() string { return "id-1" }
