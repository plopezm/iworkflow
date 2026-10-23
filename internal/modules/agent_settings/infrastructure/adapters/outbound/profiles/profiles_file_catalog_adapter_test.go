package profiles

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
)

func TestParseActiveAndSortedNames(t *testing.T) {
	content := []byte(`{
		"kind": "gentle-pi.agent_model_profiles",
		"version": 1,
		"active": "Claude",
		"profiles": {"GPT": {}, "Claude": {}}
	}`)

	catalog, err := parseProfilesFile(content)

	require.NoError(t, err)
	require.NotNil(t, catalog.Active)
	require.Equal(t, "Claude", *catalog.Active)
	require.Equal(t, []string{"Claude", "GPT"}, catalog.Names)
}

func TestMissingActiveIsNil(t *testing.T) {
	catalog, err := parseProfilesFile([]byte(`{"kind": "gentle-pi.agent_model_profiles", "version": 1, "profiles": {}}`))

	require.NoError(t, err)
	require.Nil(t, catalog.Active)
	require.Empty(t, catalog.Names)
	require.NotNil(t, catalog.Names)
}

func TestMissingProfilesObjectIsAnEmptyCatalog(t *testing.T) {
	catalog, err := parseProfilesFile([]byte(`{"active": "Claude"}`))

	require.NoError(t, err)
	require.Empty(t, catalog.Names)
}

func TestInvalidJSONIsACatalogError(t *testing.T) {
	_, err := parseProfilesFile([]byte("not json"))

	require.ErrorIs(t, err, agenterrors.ErrCatalog)
}

func TestNullProfilesIsRejectedLikeTheRustParser(t *testing.T) {
	_, err := parseProfilesFile([]byte(`{"profiles": null}`))

	require.ErrorIs(t, err, agenterrors.ErrCatalog)
}

func TestNullDocumentIsRejected(t *testing.T) {
	_, err := parseProfilesFile([]byte("null"))

	require.ErrorIs(t, err, agenterrors.ErrCatalog)
}

func TestNonStringActiveIsRejected(t *testing.T) {
	_, err := parseProfilesFile([]byte(`{"active": 7, "profiles": {}}`))

	require.ErrorIs(t, err, agenterrors.ErrCatalog)
}

func TestMissingFileIsAnEmptyCatalog(t *testing.T) {
	adapter := NewProfilesFileCatalogAdapter(t.TempDir())

	catalog, err := adapter.List(context.Background())

	require.NoError(t, err)
	require.Nil(t, catalog.Active)
	require.NotNil(t, catalog.Names)
	require.Empty(t, catalog.Names)
}

func TestListReadsProfilesJSONFromTheConfigHome(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, profilesFileName),
		[]byte(`{"active": "Gentleman", "profiles": {"Gentleman": {}}}`), 0o600))
	adapter := NewProfilesFileCatalogAdapter(home)

	catalog, err := adapter.List(context.Background())

	require.NoError(t, err)
	require.NotNil(t, catalog.Active)
	require.Equal(t, "Gentleman", *catalog.Active)
	require.Equal(t, []string{"Gentleman"}, catalog.Names)
}

func TestCancelledContextStopsTheRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewProfilesFileCatalogAdapter(t.TempDir()).List(ctx)

	require.ErrorIs(t, err, context.Canceled)
}
