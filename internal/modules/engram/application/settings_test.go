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

type fakeSettingsStore struct {
	saved   string
	err     error
	written []string
}

func (f *fakeSettingsStore) SavedURL(context.Context) (string, error) { return f.saved, f.err }
func (f *fakeSettingsStore) SaveURL(_ context.Context, url string) error {
	if f.err != nil {
		return f.err
	}
	f.saved = url
	f.written = append(f.written, url)
	return nil
}

func TestGetServerSettingsPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		saved, env string
		want       models.ServerSettings
	}{
		{"saved wins", "http://10.0.0.5:9000/", "http://127.0.0.1:1", models.ServerSettings{URL: "http://10.0.0.5:9000", Source: models.SourceSaved}},
		{"environment when nothing saved", "", "http://127.0.0.1:1", models.ServerSettings{URL: "http://127.0.0.1:1", Source: models.SourceEnvironment}},
		{"invalid saved falls to environment", "nope", "http://127.0.0.1:1", models.ServerSettings{URL: "http://127.0.0.1:1", Source: models.SourceEnvironment}},
		{"default", "", "", models.ServerSettings{URL: models.DefaultServerURL, Source: models.SourceDefault}},
		{"invalid environment is ignored", "", "ftp://x", models.ServerSettings{URL: models.DefaultServerURL, Source: models.SourceDefault}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewGetServerSettings(&fakeSettingsStore{saved: tc.saved}, tc.env).Execute(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
	_, err := NewGetServerSettings(&fakeSettingsStore{err: errors.New("db")}, "").Execute(context.Background())
	assert.Error(t, err)
}

func TestSaveServerSettings(t *testing.T) {
	store := &fakeSettingsStore{}
	uc := NewSaveServerSettings(store)

	require.NoError(t, uc.Execute(context.Background(), "  http://127.0.0.1:8000/  "))
	assert.Equal(t, "http://127.0.0.1:8000", store.saved)

	require.NoError(t, uc.Execute(context.Background(), "   "))
	assert.Equal(t, "", store.saved, "blank restores the default")

	before := len(store.written)
	for _, bad := range []string{"ftp://x", "http://", "http://user:pw@host:1", "not a url"} {
		err := uc.Execute(context.Background(), bad)
		assert.ErrorIs(t, err, engramerrors.ErrInvalidServerURL, bad)
		assert.NotContains(t, err.Error(), "pw")
	}
	assert.Len(t, store.written, before, "invalid input never reaches storage")
}

type fakeSwitcher struct {
	got string
	err error
}

func (f *fakeSwitcher) Switch(url string) error { f.got = url; return f.err }

func TestApplyServerSettingsSwitchesThenStartsOnlyWhenOff(t *testing.T) {
	store := &fakeSettingsStore{saved: "http://127.0.0.1:9100"}
	switcher := &fakeSwitcher{}
	process := &fakeProcess{healthyAfter: 2}
	apply := NewApplyServerSettings(NewGetServerSettings(store, ""), switcher, NewEnsureRunning(process, noSleep, time.Now))

	settings, outcome, err := apply.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:9100", switcher.got)
	assert.Equal(t, settings.URL, switcher.got)
	assert.Equal(t, models.OutcomeStarted, outcome)
	assert.Equal(t, []string{"9100"}, process.started)

	running := &fakeProcess{healthyAfter: 1}
	_, outcome, err = NewApplyServerSettings(NewGetServerSettings(store, ""), switcher, NewEnsureRunning(running, noSleep, time.Now)).Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, models.OutcomeRunning, outcome)
	assert.Empty(t, running.started)
}

func TestApplyServerSettingsKeepsTheAddressWhenTheStartFails(t *testing.T) {
	switcher := &fakeSwitcher{}
	apply := NewApplyServerSettings(NewGetServerSettings(&fakeSettingsStore{}, ""), switcher, NewEnsureRunning(&fakeProcess{startErr: errors.New("exec")}, noSleep, time.Now))

	settings, _, err := apply.Execute(context.Background())

	require.Error(t, err)
	assert.Equal(t, models.DefaultServerURL, switcher.got)
	assert.Equal(t, models.DefaultServerURL, settings.URL)

	_, err = NewApplyServerSettings(NewGetServerSettings(&fakeSettingsStore{}, ""), &fakeSwitcher{err: errors.New("bad")}, NewEnsureRunning(&fakeProcess{}, noSleep, time.Now)).Apply(context.Background())
	assert.Error(t, err)
}
