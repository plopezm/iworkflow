package application_test

import (
	"context"

	"iworkflow/internal/modules/data_home/domain/models"
)

// fakeMover is an in-memory outbound.HomeMoverPort with error injection. It lives next to the use
// case tests because _test files cannot be imported across packages.
type fakeMover struct {
	status     models.HomeStatus
	statusErr  error
	requestErr error
	cancelErr  error
	requested  []string
	cancels    int
}

func (f *fakeMover) Status(context.Context) (models.HomeStatus, error) {
	return f.status, f.statusErr
}

func (f *fakeMover) Request(_ context.Context, destination string) error {
	if f.requestErr == nil {
		f.requested = append(f.requested, destination)
	}
	return f.requestErr
}

func (f *fakeMover) Cancel(context.Context) error {
	f.cancels++
	return f.cancelErr
}

// fakeTasks reports whether a task is active.
type fakeTasks struct {
	active bool
	err    error
}

func (f *fakeTasks) HasActive(context.Context) (bool, error) {
	return f.active, f.err
}

// fakeRestarter counts the restart requests.
type fakeRestarter struct {
	requests int
}

func (f *fakeRestarter) RequestRestart() {
	f.requests++
}
