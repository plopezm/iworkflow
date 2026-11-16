package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
)

type fakeProcess struct {
	healthyAfter int // Healthy answers true from this call number on; 0 means never
	healthyCalls int
	started      []string
	startErr     error
}

func (f *fakeProcess) Healthy(context.Context, string) bool {
	f.healthyCalls++
	return f.healthyAfter > 0 && f.healthyCalls >= f.healthyAfter
}

func (f *fakeProcess) Start(_ context.Context, port string) error {
	f.started = append(f.started, port)
	return f.startErr
}

func noSleep(context.Context, time.Duration) error { return nil }

func TestEnsureRunning(t *testing.T) {
	local := models.ServerSettings{URL: "http://127.0.0.1:7437", Source: models.SourceDefault}
	t.Run("already running is left alone", func(t *testing.T) {
		process := &fakeProcess{healthyAfter: 1}
		outcome, err := NewEnsureRunning(process, noSleep, time.Now).Execute(context.Background(), local)
		require.NoError(t, err)
		assert.Equal(t, models.OutcomeRunning, outcome)
		assert.Empty(t, process.started)
	})
	t.Run("starts when off and waits for it", func(t *testing.T) {
		process := &fakeProcess{healthyAfter: 3}
		outcome, err := NewEnsureRunning(process, noSleep, time.Now).Execute(context.Background(), local)
		require.NoError(t, err)
		assert.Equal(t, models.OutcomeStarted, outcome)
		assert.Equal(t, []string{"7437"}, process.started)
	})
	t.Run("never starts for environment, remote or portless addresses", func(t *testing.T) {
		for _, settings := range []models.ServerSettings{
			{URL: "http://127.0.0.1:7437", Source: models.SourceEnvironment},
			{URL: "http://10.0.0.5:7437", Source: models.SourceSaved},
			{URL: "http://127.0.0.1", Source: models.SourceSaved},
		} {
			process := &fakeProcess{}
			outcome, err := NewEnsureRunning(process, noSleep, time.Now).Execute(context.Background(), settings)
			require.NoError(t, err)
			assert.Equal(t, models.OutcomeSkipped, outcome, settings.URL)
			assert.Empty(t, process.started)
		}
	})
	t.Run("start failure and silent server are errors", func(t *testing.T) {
		_, err := NewEnsureRunning(&fakeProcess{startErr: errors.New("exec")}, noSleep, time.Now).Execute(context.Background(), local)
		assert.Error(t, err)
		_, err = NewEnsureRunning(&fakeProcess{}, noSleep, time.Now).Execute(context.Background(), local)
		assert.ErrorIs(t, err, engramerrors.ErrStartFailed)
	})
	t.Run("cancelled wait stops", func(t *testing.T) {
		cancelled := func(context.Context, time.Duration) error { return context.Canceled }
		_, err := NewEnsureRunning(&fakeProcess{}, cancelled, time.Now).Execute(context.Background(), local)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestEnsureRunningDoesNotStartASecondTimeWhileTheFirstIsStillInitializing(t *testing.T) {
	local := models.ServerSettings{URL: "http://127.0.0.1:7437", Source: models.SourceDefault}
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time { return clock }
	process := &fakeProcess{} // never answers: the first start times out
	uc := NewEnsureRunning(process, noSleep, now)

	_, err := uc.Execute(context.Background(), local)
	require.ErrorIs(t, err, engramerrors.ErrStartFailed)
	assert.Equal(t, []string{"7437"}, process.started)

	clock = clock.Add(10 * time.Second) // still inside the grace period: only wait
	_, err = uc.Execute(context.Background(), local)
	require.ErrorIs(t, err, engramerrors.ErrStartFailed)
	assert.Equal(t, []string{"7437"}, process.started, "a second Engram was launched during the grace period")

	clock = clock.Add(time.Minute) // grace over: it may start again
	_, err = uc.Execute(context.Background(), local)
	require.ErrorIs(t, err, engramerrors.ErrStartFailed)
	assert.Equal(t, []string{"7437", "7437"}, process.started)
}

func TestEnsureRunningRetriesAtOnceWhenTheLaunchItselfFailed(t *testing.T) {
	local := models.ServerSettings{URL: "http://127.0.0.1:7437", Source: models.SourceDefault}
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	process := &fakeProcess{startErr: errors.New("exec: engram not found")}
	uc := NewEnsureRunning(process, noSleep, func() time.Time { return clock })

	_, err := uc.Execute(context.Background(), local)
	require.Error(t, err)
	process.startErr = nil // the user installed engram
	process.healthyAfter = process.healthyCalls + 2
	clock = clock.Add(time.Second) // well inside the grace period

	outcome, err := uc.Execute(context.Background(), local)

	require.NoError(t, err)
	assert.Equal(t, models.OutcomeStarted, outcome)
	assert.Equal(t, []string{"7437", "7437"}, process.started, "the retry after a failed launch must launch again")
}
