package desktop

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extractInto extracts data into base/staging and returns the staging folder and the extraction error.
func extractInto(t *testing.T, data []byte, maxBytes int64) (string, string, error) {
	t.Helper()
	base := t.TempDir()
	staging := filepath.Join(base, "staging")
	require.NoError(t, os.Mkdir(staging, 0o755))
	err := extractArchive(bytes.NewReader(data), staging, maxBytes)
	return base, staging, err
}

func TestExtractArchive(t *testing.T) {
	tests := []struct {
		name      string
		entries   []tarEntry
		maxBytes  int64
		wantError string
		check     func(t *testing.T, base string, staging string)
	}{
		{
			name:    "strips the top folder and keeps file modes",
			entries: standardEntries(testTop),
			check: func(t *testing.T, base, staging string) {
				t.Helper()
				assert.Equal(t, []string{"iworkflow-desktop", "iworkflow.desktop", "iworkflow.png", "server"}, listing(t, staging))
				if runtime.GOOS == "windows" {
					return
				}
				info, err := os.Stat(filepath.Join(staging, "iworkflow-desktop"))
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
				info, err = os.Stat(filepath.Join(staging, "server", "server.bin"))
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
			},
		},
		{
			name: "symlink inside the folder is accepted",
			entries: append(standardEntries(testTop), tarEntry{
				name: testTop + "/current", typeflag: tar.TypeSymlink, linkname: "server/server.bin",
			}),
			check: func(t *testing.T, base, staging string) {
				t.Helper()
				target, err := os.Readlink(filepath.Join(staging, "current"))
				require.NoError(t, err)
				assert.Equal(t, filepath.FromSlash("server/server.bin"), target)
			},
		},
		{
			name: "hard link to a file of the archive is accepted",
			entries: append(standardEntries(testTop), tarEntry{
				name: testTop + "/alias", typeflag: tar.TypeLink, linkname: testTop + "/iworkflow.png",
			}),
			check: func(t *testing.T, base, staging string) {
				t.Helper()
				got, err := os.ReadFile(filepath.Join(staging, "alias"))
				require.NoError(t, err)
				assert.Equal(t, "new-png", string(got))
			},
		},
		{
			name:      "parent reference is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: testTop + "/../../evil", typeflag: tar.TypeReg, mode: 0o644, body: "x"}),
			wantError: "parent reference",
		},
		{
			name:      "absolute name is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: "/etc/evil", typeflag: tar.TypeReg, mode: 0o644, body: "x"}),
			wantError: "absolute",
		},
		{
			name:      "backslash in name is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: testTop + "\\evil", typeflag: tar.TypeReg, mode: 0o644, body: "x"}),
			wantError: "backslash",
		},
		{
			name:      "symlink escaping the folder is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: testTop + "/link", typeflag: tar.TypeSymlink, linkname: "../../outside"}),
			wantError: "leaves the staging folder",
		},
		{
			name:      "absolute symlink target is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: testTop + "/link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"}),
			wantError: "absolute",
		},
		{
			name:      "hard link source outside the top folder is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: testTop + "/alias", typeflag: tar.TypeLink, linkname: "other/file"}),
			wantError: "outside the top folder",
		},
		{
			name:      "hard link escaping the folder is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: testTop + "/alias", typeflag: tar.TypeLink, linkname: testTop + "/../../etc/passwd"}),
			wantError: "parent reference",
		},
		{
			name:      "hard link to a directory is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: testTop + "/alias", typeflag: tar.TypeLink, linkname: testTop + "/server"}),
			wantError: "not a regular file",
		},
		{
			name:      "device entry is rejected",
			entries:   append(standardEntries(testTop), tarEntry{name: testTop + "/dev", typeflag: tar.TypeChar}),
			wantError: "unsupported entry type",
		},
		{
			name: "entry outside the single top folder is rejected",
			entries: append(standardEntries(testTop), tarEntry{
				name: "other/file", typeflag: tar.TypeReg, mode: 0o644, body: "x",
			}),
			wantError: "single top folder",
		},
		{
			name:      "extraction over the size cap is rejected",
			entries:   standardEntries(testTop),
			maxBytes:  8,
			wantError: "more than 8 bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildTarGz(t, tt.entries)
			maxBytes := tt.maxBytes
			if maxBytes == 0 {
				maxBytes = DefaultMaxExtractBytes
			}

			base, staging, err := extractInto(t, data, maxBytes)

			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				// Nothing may be written outside the staging folder.
				assert.Equal(t, []string{"staging"}, listing(t, base))
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, base, staging)
			}
		})
	}
}
