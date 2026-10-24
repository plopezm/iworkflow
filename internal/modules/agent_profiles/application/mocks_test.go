package application_test

import (
	"context"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// fakeProfileStore is an in-memory ProfileStorePort. Set the *Err fields to inject failures. When
// yield is true, every call gives other goroutines a chance to run, which exposes lost updates.
type fakeProfileStore struct {
	mu      sync.Mutex
	store   models.ProfileStore
	loadErr error
	saveErr error
	yield   bool
	loads   int
	saves   []models.ProfileStore
}

func newFakeProfileStore(initial models.ProfileStore) *fakeProfileStore {
	return &fakeProfileStore{store: cloneStore(initial)}
}

func (f *fakeProfileStore) Load(ctx context.Context) (models.ProfileStore, error) {
	if err := ctx.Err(); err != nil {
		return models.ProfileStore{}, err
	}
	f.mu.Lock()
	f.loads++
	loaded := cloneStore(f.store)
	loadErr := f.loadErr
	f.mu.Unlock()
	if f.yield {
		runtime.Gosched()
	}
	if loadErr != nil {
		return models.ProfileStore{}, loadErr
	}
	return loaded, nil
}

func (f *fakeProfileStore) Save(ctx context.Context, store models.ProfileStore) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.yield {
		runtime.Gosched()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saves = append(f.saves, cloneStore(store))
	f.store = cloneStore(store)
	return nil
}

// snapshot returns the store as it was last saved.
func (f *fakeProfileStore) snapshot() models.ProfileStore {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneStore(f.store)
}

// savedCount returns how many times Save succeeded.
func (f *fakeProfileStore) savedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.saves)
}

// fakeGlobalRouting is an in-memory GlobalRoutingPort.
type fakeGlobalRouting struct {
	routing map[string]models.RoutingEntry
	loadErr error
	saveErr error
	saves   int
}

func (f *fakeGlobalRouting) Load(_ context.Context) (map[string]models.RoutingEntry, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return models.CloneRouting(f.routing), nil
}

func (f *fakeGlobalRouting) Save(_ context.Context, routing map[string]models.RoutingEntry) error {
	f.saves++
	if f.saveErr != nil {
		return f.saveErr
	}
	f.routing = models.CloneRouting(routing)
	return nil
}

// fakeAgentRoles is an in-memory AgentRolePort.
type fakeAgentRoles struct {
	roles     []models.AgentRole
	listErr   error
	createErr error
	created   []models.NewAgentRole
}

func (f *fakeAgentRoles) List(_ context.Context) ([]models.AgentRole, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.roles, nil
}

func (f *fakeAgentRoles) Create(_ context.Context, role models.NewAgentRole) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, role)
	return nil
}

// fakeModelOptions is a ModelOptionsPort that returns a fixed list.
type fakeModelOptions struct {
	options []outbound.ModelOption
	err     error
}

func (f *fakeModelOptions) List(_ context.Context) ([]outbound.ModelOption, error) {
	return f.options, f.err
}

// fakeExchange is a ProfileExchangePort. serialized is returned by SerializeProfile, parsed by
// ParseProfile; parseErr replaces the parse result.
type fakeExchange struct {
	serialized string
	parsed     models.Profile
	parseErr   error
	serializes int
	parses     int
}

func (f *fakeExchange) SerializeProfile(_ context.Context, profile models.Profile) (string, error) {
	f.serializes++
	f.serialized = profile.Name.String()
	return f.serialized, nil
}

func (f *fakeExchange) ParseProfile(_ context.Context, _ string) (models.Profile, error) {
	f.parses++
	if f.parseErr != nil {
		return models.Profile{}, f.parseErr
	}
	return f.parsed, nil
}

// cloneStore deep-copies a store so fakes never share mutable state with their callers.
func cloneStore(store models.ProfileStore) models.ProfileStore {
	var active *models.ProfileName
	if store.Active != nil {
		copied := *store.Active
		active = &copied
	}
	profiles := make([]models.Profile, 0, len(store.Profiles))
	for _, profile := range store.Profiles {
		profiles = append(profiles, models.NewProfile(profile.Name, models.CloneRouting(profile.Routing)))
	}
	return models.NewProfileStore(active, profiles)
}

// profileNames returns the names of the profiles in order.
func profileNames(profiles []models.Profile) []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.Name.String())
	}
	return names
}

func profileName(t *testing.T, raw string) models.ProfileName {
	t.Helper()
	name, err := models.ParseProfileName(raw)
	require.NoError(t, err)
	return name
}

func ptr(value string) *string {
	return &value
}

// routingOf returns a routing with one agent that uses model and the "high" thinking level.
func routingOf(model string) map[string]models.RoutingEntry {
	return map[string]models.RoutingEntry{"sdd-apply": {Model: ptr(model), Thinking: ptr("high")}}
}

func profileOf(t *testing.T, raw string, model string) models.Profile {
	t.Helper()
	return models.NewProfile(profileName(t, raw), routingOf(model))
}

// storeOf builds a store. An empty active means no active profile.
func storeOf(t *testing.T, active string, profiles ...models.Profile) models.ProfileStore {
	t.Helper()
	var activeName *models.ProfileName
	if active != "" {
		parsed := profileName(t, active)
		activeName = &parsed
	}
	return models.NewProfileStore(activeName, profiles)
}

// errIO is a shared injected failure for the "propagates error" cases.
var errIO = agenterrors.IO("x")
