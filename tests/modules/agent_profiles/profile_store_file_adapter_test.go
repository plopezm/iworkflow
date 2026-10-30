package agent_profiles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/profiles"
)

func profileNameOf(t *testing.T, raw string) models.ProfileName {
	t.Helper()
	name, err := models.ParseProfileName(raw)
	require.NoError(t, err)
	return name
}

func entryOf(model *string, thinking *string) models.RoutingEntry {
	return models.RoutingEntry{Model: model, Thinking: thinking}
}

func strPtr(value string) *string {
	return &value
}

func sampleStore(t *testing.T) models.ProfileStore {
	t.Helper()
	active := profileNameOf(t, "Claude")
	return models.NewProfileStore(&active, []models.Profile{
		models.NewProfile(active, map[string]models.RoutingEntry{
			"orchestrator": entryOf(strPtr("a/b"), strPtr("high")),
			"sdd-apply":    entryOf(strPtr("a/c"), nil),
			"sdd-verify":   entryOf(nil, strPtr("low")),
		}),
		models.NewProfile(profileNameOf(t, "Alpha"), nil),
	})
}

func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestProfileStoreMissingFileLoadsEmptyStore(t *testing.T) {
	adapter := profiles.NewProfileStoreFileAdapter(t.TempDir())

	loaded, err := adapter.Load(t.Context())

	require.NoError(t, err)
	require.Equal(t, models.NewProfileStore(nil, nil), loaded)
}

func TestProfileStoreSaveThenLoadRoundTripsSorted(t *testing.T) {
	dir := t.TempDir()
	adapter := profiles.NewProfileStoreFileAdapter(dir)
	require.NoError(t, adapter.Save(t.Context(), sampleStore(t)))

	loaded, err := adapter.Load(t.Context())

	require.NoError(t, err)
	expected := sampleStore(t)
	expected.Profiles = []models.Profile{expected.Profiles[1], expected.Profiles[0]}
	require.Equal(t, expected, loaded)
	raw, err := os.ReadFile(filepath.Join(dir, "profiles.json"))
	require.NoError(t, err)
	content := string(raw)
	require.Contains(t, content, `"kind": "gentle-pi.agent_model_profiles"`)
	require.Less(t, strings.Index(content, `"Alpha"`), strings.Index(content, `"Claude": {`))
	require.True(t, strings.HasSuffix(content, "\n"))
}

func TestProfileStoreSaveOmitsActiveWhenNoneAndCreatesDir(t *testing.T) {
	nested := filepath.Join(t.TempDir(), "a", "b")
	adapter := profiles.NewProfileStoreFileAdapter(nested)
	store := sampleStore(t)
	store.Active = nil
	require.NoError(t, adapter.Save(t.Context(), store))

	raw, err := os.ReadFile(filepath.Join(nested, "profiles.json"))
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"active"`)
	loaded, err := adapter.Load(t.Context())
	require.NoError(t, err)
	require.Nil(t, loaded.Active)
}

func TestProfileStoreSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	adapter := profiles.NewProfileStoreFileAdapter(dir)
	require.NoError(t, adapter.Save(t.Context(), sampleStore(t)))
	require.NoError(t, adapter.Save(t.Context(), sampleStore(t)))

	require.Equal(t, []string{"profiles.json"}, leftovers(t, dir))
}

func TestProfileStoreCorruptJSONIsStoreError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "profiles.json"), []byte("{not json"), 0o600))
	adapter := profiles.NewProfileStoreFileAdapter(dir)

	_, err := adapter.Load(t.Context())

	require.ErrorIs(t, err, agenterrors.Store(""))
}

func TestProfileStoreInvalidEntryIsStoreErrorNamingProfileAndAgent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "profiles.json"),
		[]byte(`{"profiles":{"Ok":{"sdd-apply":{"thinking":"ultra"}}}}`), 0o600))
	adapter := profiles.NewProfileStoreFileAdapter(dir)

	_, err := adapter.Load(t.Context())

	require.ErrorIs(t, err, agenterrors.Store(""))
	require.ErrorContains(t, err, "perfil `Ok`")
	require.ErrorContains(t, err, "agente `sdd-apply`")
}

func TestProfileStoreInvalidProfileNameIsStoreErrorNamingIt(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(`{"profiles":{"bad name":{}}}`), 0o600))
	adapter := profiles.NewProfileStoreFileAdapter(dir)

	_, err := adapter.Load(t.Context())

	require.ErrorIs(t, err, agenterrors.Store(""))
	require.ErrorContains(t, err, "bad name")
}

func TestProfileStoreLegacyStringEntryIsModelOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "profiles.json"),
		[]byte(`{"active":"Ok","profiles":{"Ok":{"orchestrator":"p/m"}}}`), 0o600))
	adapter := profiles.NewProfileStoreFileAdapter(dir)

	loaded, err := adapter.Load(t.Context())

	require.NoError(t, err)
	require.Equal(t, entryOf(strPtr("p/m"), nil), loaded.Profiles[0].Routing["orchestrator"])
	require.Equal(t, profileNameOf(t, "Ok"), *loaded.Active)
}
