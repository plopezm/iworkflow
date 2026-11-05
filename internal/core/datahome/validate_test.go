package datahome_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/datahome"
)

func dataFolder(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "iwf")
	writeFile(t, filepath.Join(root, "workspaces.db"), "db", 0o600)
	return root
}

func TestValidateAcceptsANewFolderAndAnEmptyOne(t *testing.T) {
	source := dataFolder(t)
	parent := t.TempDir()
	empty := filepath.Join(parent, "empty")
	require.NoError(t, os.Mkdir(empty, 0o755))

	require.NoError(t, datahome.Validate(source, filepath.Join(parent, "new", "deeper")))
	require.NoError(t, datahome.Validate(source, empty))
}

func TestValidateRejections(t *testing.T) {
	source := dataFolder(t)
	parent := t.TempDir()
	nonEmpty := filepath.Join(parent, "full")
	writeFile(t, filepath.Join(nonEmpty, "x"), "x", 0o644)
	aFile := filepath.Join(parent, "file")
	writeFile(t, aFile, "x", 0o644)
	notData := filepath.Join(parent, "not-data")
	require.NoError(t, os.MkdirAll(notData, 0o755))
	home := t.TempDir()

	cases := []struct {
		name        string
		source, dst string
		home        string
		want        error
	}{
		{"relative", source, "relative/dir", home, datahome.ErrNotAbsolute},
		{"same folder", source, source, home, datahome.ErrSameFolder},
		{"inside the source", source, filepath.Join(source, "sub"), home, datahome.ErrNested},
		{"containing the source", source, filepath.Dir(source), home, datahome.ErrNested},
		{"not empty", source, nonEmpty, home, datahome.ErrNotEmpty},
		{"is a file", source, aFile, home, datahome.ErrNotEmpty},
		{"source is not a data folder", notData, filepath.Join(parent, "x"), home, datahome.ErrNotDataFolder},
		{"source is the user home", home, filepath.Join(parent, "y"), home, datahome.ErrUnsafeSource},
		{"source is a filesystem root", string(filepath.Separator), filepath.Join(parent, "z"), home, datahome.ErrUnsafeSource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "source is the user home" {
				writeFile(t, filepath.Join(home, "workspaces.db"), "db", 0o600)
			}
			require.ErrorIs(t, datahome.ValidateWithHome(tc.source, tc.dst, tc.home), tc.want)
		})
	}
}

func TestValidateRejectsADestinationReachedThroughASymlinkIntoTheSource(t *testing.T) {
	source := dataFolder(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(source, link); err != nil {
		t.Skip("symlinks unavailable")
	}

	require.ErrorIs(t, datahome.Validate(source, filepath.Join(link, "inner")), datahome.ErrNested)
}

func TestValidateRejectsAnUnwritableParent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	source := dataFolder(t)
	locked := filepath.Join(t.TempDir(), "locked")
	require.NoError(t, os.Mkdir(locked, 0o500))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	require.ErrorIs(t, datahome.Validate(source, filepath.Join(locked, "iwf")), datahome.ErrNotWritable)
}

func TestSizeOfCountsRegularFiles(t *testing.T) {
	source := dataFolder(t)
	writeFile(t, filepath.Join(source, "a", "b"), "12345", 0o644)

	manifest, err := datahome.Scan(source, nil)
	require.NoError(t, err)

	require.Equal(t, int64(2+5), manifest.TotalSize())
}

func TestFreeSpaceOfAnExistingFolderIsPositive(t *testing.T) {
	free, err := datahome.FreeSpace(t.TempDir())

	require.NoError(t, err)
	require.Positive(t, free)
}
