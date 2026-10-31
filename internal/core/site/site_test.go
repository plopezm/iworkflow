package site

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func base() Options {
	return Options{
		SiteRoot: "target/site",
		SiteAddr: netip.MustParseAddrPort("127.0.0.1:3000"),
	}
}

func TestSiteDirNextToTheExecutableIsUsed(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "site"), 0o750))

	got := ApplyPackagedDefaults(base(), dir, false)

	assert.Equal(t, filepath.Join(dir, "site"), got.SiteRoot)
}

func TestExplicitSiteRootWins(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "site"), 0o750))

	got := ApplyPackagedDefaults(base(), dir, true)

	assert.Equal(t, "target/site", got.SiteRoot)
}

func TestMissingSiteDirKeepsDefaultRoot(t *testing.T) {
	got := ApplyPackagedDefaults(base(), t.TempDir(), false)

	assert.Equal(t, "target/site", got.SiteRoot)
}

func TestNoExecutableDirKeepsDefaultRoot(t *testing.T) {
	got := ApplyPackagedDefaults(base(), "", false)

	assert.Equal(t, "target/site", got.SiteRoot)
}

func TestPackagedPortIsUsedWhenTheAddressIsNotPinned(t *testing.T) {
	got := ApplyPackagedPort(base(), false)

	assert.Equal(t, uint16(9000), got.SiteAddr.Port())
	assert.True(t, got.SiteAddr.Addr().IsLoopback())
}

func TestPinnedAddressKeepsItsPort(t *testing.T) {
	got := ApplyPackagedPort(base(), true)

	assert.Equal(t, uint16(3000), got.SiteAddr.Port())
}

func TestBundledSiteRootReportsOnlyExistingDirectories(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "site"), []byte("not a dir"), 0o600))

	_, ok := BundledSiteRoot(dir)

	assert.False(t, ok)
}
