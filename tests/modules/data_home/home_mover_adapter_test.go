package data_home_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/datahome"
	domainerrors "iworkflow/internal/modules/data_home/domain/errors"
	"iworkflow/internal/modules/data_home/domain/models"
	"iworkflow/internal/modules/data_home/infrastructure/adapters/outbound/system"
)

// env is a fake environment: the adapter reads only what it is given, so the tests never touch the
// configuration of the machine running them.
type env map[string]string

func (e env) lookup(key string) string { return e[key] }

// fixture is a home folder with a configuration folder, a data folder and a free destination.
type fixture struct {
	env         env
	dataFolder  string
	destination string
	pointer     string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".iworkflow"), 0o755))
	dataFolder := filepath.Join(home, ".iworkflow")
	require.NoError(t, os.WriteFile(filepath.Join(dataFolder, datahome.DatabaseFile), []byte("db"), 0o600))
	return fixture{
		env:         env{"HOME": home, "XDG_CONFIG_HOME": filepath.Join(root, "config")},
		dataFolder:  dataFolder,
		destination: filepath.Join(root, "moved"),
		pointer:     filepath.Join(root, "config", "iworkflow", "data-home"),
	}
}

func (f fixture) adapter() *system.HomeMoverAdapter {
	fixedNow := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	return system.NewHomeMoverAdapter(runtime.GOOS, f.env.lookup, func() time.Time { return fixedNow }, func(string, ...any) {})
}

func TestStatusDefaultsToTheHomeFolderWhenNothingIsSet(t *testing.T) {
	f := newFixture(t)

	status, err := f.adapter().Status(context.Background())

	require.NoError(t, err)
	require.Equal(t, f.dataFolder, status.Current)
	require.Equal(t, models.SourceDefault, status.Source)
	require.False(t, status.EnvLocked)
	require.Nil(t, status.Pending)
	require.Nil(t, status.Last)
}

func TestStatusIsLockedWhenIWFHomeIsSet(t *testing.T) {
	f := newFixture(t)
	f.env["IWF_HOME"] = f.destination

	status, err := f.adapter().Status(context.Background())

	require.NoError(t, err)
	require.Equal(t, f.destination, status.Current)
	require.Equal(t, models.SourceEnv, status.Source)
	require.True(t, status.EnvLocked)
}

func TestStatusReadsTheFolderFromThePointer(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, datahome.Write(f.pointer, f.destination))

	status, err := f.adapter().Status(context.Background())

	require.NoError(t, err)
	require.Equal(t, f.destination, status.Current)
	require.Equal(t, models.SourcePointer, status.Source)
}

func TestStatusFailsWhenThePointerIsCorrupt(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(f.pointer), 0o755))
	require.NoError(t, os.WriteFile(f.pointer, []byte("relative/path\n"), 0o600))

	_, err := f.adapter().Status(context.Background())

	require.True(t, errors.Is(err, domainerrors.ErrUnavailable))
	require.True(t, errors.Is(err, datahome.ErrCorruptPointer))
}

func TestRequestRecordsTheMoveAndCancelWithdrawsIt(t *testing.T) {
	f := newFixture(t)
	adapter := f.adapter()

	require.NoError(t, adapter.Request(context.Background(), f.destination))

	status, err := adapter.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, &models.PendingMove{From: f.dataFolder, To: f.destination}, status.Pending)
	require.Equal(t, f.dataFolder, status.Current, "a request does not switch the folder by itself")
	require.NoDirExists(t, f.destination, "a request does not copy anything")

	require.NoError(t, adapter.Cancel(context.Background()))
	status, err = adapter.Status(context.Background())
	require.NoError(t, err)
	require.Nil(t, status.Pending)
}

func TestRequestRefusesASecondPendingMove(t *testing.T) {
	f := newFixture(t)
	adapter := f.adapter()
	require.NoError(t, adapter.Request(context.Background(), f.destination))

	err := adapter.Request(context.Background(), filepath.Join(f.destination, "again"))

	require.True(t, errors.Is(err, domainerrors.ErrMovePending))
}

func TestRequestMapsEveryRefusedDestinationToItsReason(t *testing.T) {
	f := newFixture(t)
	notEmpty := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(notEmpty, "keep.txt"), []byte("x"), 0o600))
	inside := filepath.Join(f.dataFolder, "inside")

	cases := []struct {
		name        string
		destination string
		reason      domainerrors.DestinationReason
	}{
		{name: "relative", destination: "relative/path", reason: domainerrors.ReasonRelative},
		{name: "same folder", destination: f.dataFolder, reason: domainerrors.ReasonSame},
		{name: "nested inside the source", destination: inside, reason: domainerrors.ReasonNested},
		{name: "not empty", destination: notEmpty, reason: domainerrors.ReasonNotEmpty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := f.adapter().Request(context.Background(), tc.destination)

			var destinationErr *domainerrors.DestinationError
			require.True(t, errors.As(err, &destinationErr), "got %v", err)
			require.Equal(t, tc.reason, destinationErr.Reason)
		})
	}
}

func TestRequestRefusesAFolderWithoutADatabase(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, os.Remove(filepath.Join(f.dataFolder, datahome.DatabaseFile)))

	err := f.adapter().Request(context.Background(), f.destination)

	var destinationErr *domainerrors.DestinationError
	require.True(t, errors.As(err, &destinationErr))
	require.Equal(t, domainerrors.ReasonSourceMissing, destinationErr.Reason)
}

func TestEveryMethodFailsWhenTheMoverCannotBeBuilt(t *testing.T) {
	f := newFixture(t)
	f.env = env{} // no HOME and no XDG_CONFIG_HOME: there is no configuration folder
	adapter := f.adapter()

	_, statusErr := adapter.Status(context.Background())
	requestErr := adapter.Request(context.Background(), f.destination)
	cancelErr := adapter.Cancel(context.Background())

	require.True(t, errors.Is(statusErr, domainerrors.ErrUnavailable))
	require.True(t, errors.Is(requestErr, domainerrors.ErrUnavailable))
	require.True(t, errors.Is(cancelErr, domainerrors.ErrUnavailable))
}
