package agent_profiles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/routing"
)

func TestGlobalRoutingMissingFileIsEmptyMap(t *testing.T) {
	adapter := routing.NewGlobalRoutingFileAdapter(t.TempDir())

	loaded, err := adapter.Load(t.Context())

	require.NoError(t, err)
	require.Empty(t, loaded)
}

func TestGlobalRoutingLegacyStringEntryIsModelOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "models.json"),
		[]byte(`{"orchestrator":"p/m","sdd-apply":{"model":"a/b","thinking":"low"}}`), 0o600))
	adapter := routing.NewGlobalRoutingFileAdapter(dir)

	loaded, err := adapter.Load(t.Context())

	require.NoError(t, err)
	require.Equal(t, "p/m", *loaded["orchestrator"].Model)
	require.Nil(t, loaded["orchestrator"].Thinking)
	require.Equal(t, "low", *loaded["sdd-apply"].Thinking)
}

func TestGlobalRoutingSaveRoundTripsWithoutLeftovers(t *testing.T) {
	dir := t.TempDir()
	adapter := routing.NewGlobalRoutingFileAdapter(filepath.Join(dir, "x"))
	model := "a/b"
	high := "high"
	low := "low"
	routingMap := map[string]models.RoutingEntry{
		"b": {Model: &model, Thinking: &high},
		"a": {Thinking: &low},
	}
	require.NoError(t, adapter.Save(t.Context(), routingMap))
	require.NoError(t, adapter.Save(t.Context(), routingMap))

	loaded, err := adapter.Load(t.Context())

	require.NoError(t, err)
	require.Equal(t, routingMap, loaded)
	raw, err := os.ReadFile(filepath.Join(dir, "x", "models.json"))
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(string(raw), "}\n"))
	require.Less(t, strings.Index(string(raw), `"a"`), strings.Index(string(raw), `"b"`))
	entries, err := os.ReadDir(filepath.Join(dir, "x"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestGlobalRoutingCorruptJSONIsStoreError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "models.json"), []byte("[1,2]"), 0o600))
	adapter := routing.NewGlobalRoutingFileAdapter(dir)

	_, err := adapter.Load(t.Context())

	require.ErrorIs(t, err, agenterrors.Store(""))
}
