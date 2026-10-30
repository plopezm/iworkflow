package application_test

import (
	"context"
	"errors"

	"iworkflow/internal/modules/preferences/domain/models"
)

// errStorage is the storage failure injected by the fake repository.
var errStorage = errors.New("boom")

// fakeRepository is an in-memory outbound.PreferencesRepository with error injection. It lives
// next to the use case tests because _test files cannot be imported across packages.
type fakeRepository struct {
	language      *models.UILanguage
	networkAccess bool
	listenPort    *models.ListenPort
	requireToken  bool
	accessToken   *models.AccessToken
	// readErr fails every read; writeErr fails every write.
	readErr  error
	writeErr error
	// writes counts the save calls that reached the repository.
	writes int
}

// newFakeRepository starts like a fresh install: nothing chosen, token required.
func newFakeRepository() *fakeRepository {
	return &fakeRepository{requireToken: true}
}

func (f *fakeRepository) Language(context.Context) (*models.UILanguage, error) {
	return f.language, f.readErr
}

func (f *fakeRepository) SaveLanguage(_ context.Context, language models.UILanguage) error {
	f.writes++
	if f.writeErr == nil {
		f.language = &language
	}
	return f.writeErr
}

func (f *fakeRepository) NetworkAccess(context.Context) (bool, error) {
	return f.networkAccess, f.readErr
}

func (f *fakeRepository) SaveNetworkAccess(_ context.Context, enabled bool) error {
	f.writes++
	if f.writeErr == nil {
		f.networkAccess = enabled
	}
	return f.writeErr
}

func (f *fakeRepository) ListenPort(context.Context) (*models.ListenPort, error) {
	return f.listenPort, f.readErr
}

func (f *fakeRepository) SaveListenPort(_ context.Context, port models.ListenPort) error {
	f.writes++
	if f.writeErr == nil {
		f.listenPort = &port
	}
	return f.writeErr
}

func (f *fakeRepository) RequireAccessToken(context.Context) (bool, error) {
	return f.requireToken, f.readErr
}

func (f *fakeRepository) SaveRequireAccessToken(_ context.Context, required bool) error {
	f.writes++
	if f.writeErr == nil {
		f.requireToken = required
	}
	return f.writeErr
}

func (f *fakeRepository) AccessToken(context.Context) (*models.AccessToken, error) {
	return f.accessToken, f.readErr
}

func (f *fakeRepository) SaveAccessToken(_ context.Context, token models.AccessToken) error {
	f.writes++
	if f.writeErr == nil {
		f.accessToken = &token
	}
	return f.writeErr
}

// fakeNetwork is an in-memory outbound.NetworkDetector.
type fakeNetwork struct {
	interfaces []models.NetworkInterface
	preferred  string
	hasPrefer  bool
}

func (f *fakeNetwork) Interfaces(context.Context) []models.NetworkInterface {
	return f.interfaces
}

func (f *fakeNetwork) PreferredAddress(context.Context) (string, bool) {
	return f.preferred, f.hasPrefer
}

// fakeRuntime is an in-memory outbound.ServerRuntime.
type fakeRuntime struct {
	status models.ServerStatus
}

func (f *fakeRuntime) Status() models.ServerStatus {
	return f.status
}

// fakeRestarter counts restart requests.
type fakeRestarter struct {
	requests int
}

func (f *fakeRestarter) RequestRestart() {
	f.requests++
}
