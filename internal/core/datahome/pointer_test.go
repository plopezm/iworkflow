package datahome_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/datahome"
)

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestPointerPathPerPlatform(t *testing.T) {
	cases := []struct {
		name   string
		goos   string
		values map[string]string
		want   string
	}{
		{"linux xdg", "linux", map[string]string{"XDG_CONFIG_HOME": "/x/cfg", "HOME": "/home/u"}, "/x/cfg/iworkflow/data-home"},
		{"linux home", "linux", map[string]string{"HOME": "/home/u"}, "/home/u/.config/iworkflow/data-home"},
		{"relative xdg is ignored", "linux", map[string]string{"XDG_CONFIG_HOME": "rel", "HOME": "/home/u"}, "/home/u/.config/iworkflow/data-home"},
		{"darwin", "darwin", map[string]string{"HOME": "/Users/u"}, "/Users/u/Library/Application Support/iworkflow/data-home"},
		{"windows", "windows", map[string]string{"APPDATA": "/AppData/Roaming"}, filepath.Join("/AppData/Roaming", "iworkflow", "data-home")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := datahome.PointerPath(tc.goos, env(tc.values))

			require.NoError(t, err)
			require.Equal(t, filepath.FromSlash(tc.want), got)
		})
	}
}

func TestPointerPathNeedsAConfigLocation(t *testing.T) {
	_, err := datahome.PointerPath("linux", env(nil))

	require.ErrorIs(t, err, datahome.ErrNoConfigDir)
}

func TestReadMissingPointerMeansNoPointer(t *testing.T) {
	dir, err := datahome.Read(filepath.Join(t.TempDir(), "iworkflow", "data-home"))

	require.NoError(t, err)
	require.Empty(t, dir)
}

func TestWriteThenReadRoundTripsAndCreatesParents(t *testing.T) {
	pointer := filepath.Join(t.TempDir(), "a", "b", "data-home")
	target := filepath.Join(t.TempDir(), "iwf")

	require.NoError(t, datahome.Write(pointer, target))
	got, err := datahome.Read(pointer)

	require.NoError(t, err)
	require.Equal(t, target, got)
}

func TestWriteReplacesAtomicallyAndLeavesNoTemporaryFile(t *testing.T) {
	root := t.TempDir()
	pointer := filepath.Join(root, "data-home")
	require.NoError(t, datahome.Write(pointer, filepath.Join(root, "one")))
	require.NoError(t, datahome.Write(pointer, filepath.Join(root, "two")))

	got, err := datahome.Read(pointer)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "two"), got)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestWriteRejectsARelativeTarget(t *testing.T) {
	pointer := filepath.Join(t.TempDir(), "data-home")

	require.ErrorIs(t, datahome.Write(pointer, "relative/dir"), datahome.ErrNotAbsolute)
	_, err := os.Stat(pointer)
	require.ErrorIs(t, err, os.ErrNotExist, "a rejected write leaves no pointer")
}

func TestReadFailsLoudlyOnACorruptPointer(t *testing.T) {
	for name, content := range map[string]string{"relative": "rel/dir\n", "blank": "  \n"} {
		t.Run(name, func(t *testing.T) {
			pointer := filepath.Join(t.TempDir(), "data-home")
			require.NoError(t, os.WriteFile(pointer, []byte(content), 0o600))

			_, err := datahome.Read(pointer)

			require.ErrorIs(t, err, datahome.ErrCorruptPointer)
		})
	}
}

func TestReadTrimsTheTrailingNewline(t *testing.T) {
	pointer := filepath.Join(t.TempDir(), "data-home")
	target := filepath.Join(t.TempDir(), "iwf")
	require.NoError(t, os.WriteFile(pointer, []byte(target+"\n"), 0o600))

	got, err := datahome.Read(pointer)

	require.NoError(t, err)
	require.Equal(t, target, got)
}
