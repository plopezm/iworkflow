package main

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorder keeps the order of the start-up steps, so the tests can assert that the bind comes first.
type recorder struct {
	steps []string
}

func (r *recorder) bind(ctx context.Context, addr string) (net.Listener, error) {
	r.steps = append(r.steps, "bind")
	return (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
}

func (r *recorder) start(context.Context) error {
	r.steps = append(r.steps, "start")
	return nil
}

func (r *recorder) stop() {
	r.steps = append(r.steps, "stop")
}

// freeAddr returns a loopback address with a free port, so the bind of the test cannot clash with another process.
func freeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return addr
}

// The bind comes before the start: the start reconciles the tasks and sweeps the worktrees, which must not happen
// before the process owns the address.
func TestBindThenStartBindsBeforeItStarts(t *testing.T) {
	t.Parallel()
	rec := &recorder{}

	listener, err := bindThenStart(context.Background(), freeAddr(t), rec.bind, rec.start, rec.stop)

	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	assert.Equal(t, []string{"bind", "start"}, rec.steps)
}

// A second instance on a taken port fails at the bind and never starts: it does not reconcile or sweep the data the
// live instance owns, and it has nothing to stop.
func TestBindThenStartNeverStartsWhenThePortIsTaken(t *testing.T) {
	t.Parallel()
	live, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = live.Close() })
	rec := &recorder{}

	listener, err := bindThenStart(context.Background(), live.Addr().String(), rec.bind, rec.start, rec.stop)

	require.Error(t, err)
	require.ErrorContains(t, err, "bind")
	assert.Nil(t, listener)
	assert.Equal(t, []string{"bind"}, rec.steps, "start and stop must not run after a failed bind")
}

// A failed start releases what began: the listener is closed and the application is stopped exactly once.
func TestBindThenStartReleasesWhenTheStartFails(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	startErr := errors.New("reconcile failed")
	failingStart := func(context.Context) error {
		rec.steps = append(rec.steps, "start")
		return startErr
	}

	listener, err := bindThenStart(context.Background(), freeAddr(t), rec.bind, failingStart, rec.stop)

	require.ErrorIs(t, err, startErr)
	assert.Nil(t, listener)
	assert.Equal(t, []string{"bind", "start", "stop"}, rec.steps)
}

// The listener of a failed start is closed, so the port is free again for the next start.
func TestBindThenStartFreesThePortWhenTheStartFails(t *testing.T) {
	t.Parallel()
	addr := freeAddr(t)
	failingStart := func(context.Context) error { return errors.New("boom") }

	_, err := bindThenStart(context.Background(), addr, (&recorder{}).bind, failingStart, func() {})

	require.Error(t, err)
	again, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
	require.NoError(t, err, "the port must be free after a failed start")
	require.NoError(t, again.Close())
}
