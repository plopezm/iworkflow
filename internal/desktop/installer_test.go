package desktop

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testToken       = "test-token"
	testArchiveLinx = "iworkflow-1.2.0-linux-amd64.tar.gz"
	testTop         = "iworkflow-1.2.0-linux-amd64"
	testZeroDigest  = "0000000000000000000000000000000000000000000000000000000000000000"
)

var errInjected = errors.New("injected failure")

// tarEntry describes one member of an archive built by a test.
type tarEntry struct {
	name     string
	typeflag byte
	body     string
	mode     int64
	linkname string
}

// buildTarGz writes the entries as a gzip-compressed tar archive.
func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		hdr := &tar.Header{
			Name:     entry.name,
			Typeflag: entry.typeflag,
			Mode:     entry.mode,
			Linkname: entry.linkname,
			Size:     int64(len(entry.body)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if entry.typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(entry.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// standardEntries is the layout that scripts/wails_package.sh produces for Linux.
func standardEntries(top string) []tarEntry {
	return []tarEntry{
		{name: top + "/", typeflag: tar.TypeDir, mode: 0o755},
		{name: top + "/iworkflow-desktop", typeflag: tar.TypeReg, mode: 0o755, body: "new-binary"},
		{name: top + "/server/", typeflag: tar.TypeDir, mode: 0o755},
		{name: top + "/server/server.bin", typeflag: tar.TypeReg, mode: 0o644, body: "new-server"},
		{name: top + "/iworkflow.png", typeflag: tar.TypeReg, mode: 0o644, body: "new-png"},
		{name: top + "/iworkflow.desktop", typeflag: tar.TypeReg, mode: 0o644, body: "new-desktop"},
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// releaseFixture describes the GitHub release served by newFakeGitHub.
type releaseFixture struct {
	tag        string
	prerelease bool
	// assets are listed in the release and served under /files/<name>.
	assets map[string][]byte
	// blockFile names an asset that never answers until the client gives up.
	blockFile string
}

// newFakeGitHub serves the latest-release document and the assets. It requires the test token.
func newFakeGitHub(t *testing.T, rel releaseFixture) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		doc := releaseDocument{
			TagName:    rel.tag,
			HTMLURL:    "https://example.test/release",
			Prerelease: rel.prerelease,
		}
		for name := range rel.assets {
			doc.Assets = append(doc.Assets, releaseAsset{
				Name:               name,
				BrowserDownloadURL: "http://" + r.Host + "/files/" + name,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(doc); err != nil {
			t.Errorf("encode release: %v", err)
		}
	})
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/files/")
		body, ok := rel.assets[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if name == rel.blockFile {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
			return
		}
		if _, err := w.Write(body); err != nil {
			t.Errorf("write asset %s: %v", name, err)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// installLayout is an installed app: parent/iworkflow holds the executable and server/.
type installLayout struct {
	parent     string
	target     string
	executable string
}

func newInstallLayout(t *testing.T) installLayout {
	t.Helper()
	parent := t.TempDir()
	target := filepath.Join(parent, "iworkflow")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "server"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "iworkflow-desktop"), []byte("old-binary"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "server", "server.bin"), []byte("old-server"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(target, "iworkflow.png"), []byte("old-png"), 0o644))
	return installLayout{parent: parent, target: target, executable: filepath.Join(target, "iworkflow-desktop")}
}

// listing returns the sorted names in dir.
func listing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

// assertOldContents checks the files of the old install are intact.
func assertOldContents(t *testing.T, layout installLayout) {
	t.Helper()
	got, err := os.ReadFile(layout.executable)
	require.NoError(t, err)
	assert.Equal(t, "old-binary", string(got))
	server, err := os.ReadFile(filepath.Join(layout.target, "server", "server.bin"))
	require.NoError(t, err)
	assert.Equal(t, "old-server", string(server))
}

// assertInstallUntouched checks the old install is intact and the parent holds nothing but the install.
func assertInstallUntouched(t *testing.T, layout installLayout) {
	t.Helper()
	assertOldContents(t, layout)
	assert.Equal(t, []string{"iworkflow"}, listing(t, layout.parent))
}

// writeUpdateArchive writes the archive in the install parent, where ApplyUpdate expects it.
func writeUpdateArchive(t *testing.T, layout installLayout, data []byte) string {
	t.Helper()
	path := filepath.Join(layout.parent, ".iworkflow-update-test.tar.gz")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

// applyOptions returns valid options for ApplyUpdate with a fake wait and relaunch.
func applyOptions(layout installLayout, archive string, digest string, relaunched *[]string) ApplyOptions {
	return ApplyOptions{
		Archive:     archive,
		SHA256:      digest,
		Target:      layout.target,
		Executable:  layout.executable,
		ParentPID:   4242,
		WaitTimeout: time.Second,
		Relaunch: func(exe string) error {
			*relaunched = append(*relaunched, exe)
			return nil
		},
		WaitForExit: func(context.Context, int) error { return nil },
	}
}

// newTestInstaller builds an ArchiveInstaller for the layout and the fake release server.
func newTestInstaller(t *testing.T, layout installLayout, apiURL string, mutate func(*ArchiveConfig)) *ArchiveInstaller {
	t.Helper()
	cfg := ArchiveConfig{
		APIURL:      apiURL,
		AppVersion:  "1.1.0",
		GOOS:        "linux",
		GOARCH:      "amd64",
		GHToken:     testToken,
		Target:      layout.target,
		Executable:  layout.executable,
		StartHelper: func(HelperArgs) error { return nil },
		Exit:        func() {},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	inst, err := NewArchiveInstaller(cfg)
	require.NoError(t, err)
	return inst
}

// releaseAssets returns the standard release assets for version 1.2.0 with a SHA256SUMS line for the archive.
func releaseAssets(t *testing.T, archive []byte) map[string][]byte {
	t.Helper()
	sums := fmt.Sprintf("%s  %s\n", sha256Hex(archive), testArchiveLinx)
	return map[string][]byte{testArchiveLinx: archive, checksumsAssetName: []byte(sums)}
}

func TestArchiveInstallerCheck(t *testing.T) {
	archive := buildTarGz(t, standardEntries(testTop))
	otherSums := []byte(fmt.Sprintf("%s  other.tar.gz\n", sha256Hex(archive)))
	tests := []struct {
		name            string
		tag             string
		prerelease      bool
		assets          map[string][]byte
		wantAvailable   bool
		wantUnsupported bool
	}{
		{
			name:          "newer release with archive and checksum is available",
			tag:           "v1.2.0",
			assets:        releaseAssets(t, archive),
			wantAvailable: true,
		},
		{
			name:          "same version is not available",
			tag:           "v1.1.0",
			assets:        releaseAssets(t, archive),
			wantAvailable: false,
		},
		{
			name:          "older version is not available",
			tag:           "v1.0.9",
			assets:        releaseAssets(t, archive),
			wantAvailable: false,
		},
		{
			name:          "prerelease is not offered",
			tag:           "v1.2.0",
			prerelease:    true,
			assets:        releaseAssets(t, archive),
			wantAvailable: false,
		},
		{
			name:            "missing SHA256SUMS is unsupported",
			tag:             "v1.2.0",
			assets:          map[string][]byte{testArchiveLinx: archive},
			wantUnsupported: true,
		},
		{
			name:            "missing archive is unsupported",
			tag:             "v1.2.0",
			assets:          map[string][]byte{checksumsAssetName: []byte(fmt.Sprintf("%s  %s\n", sha256Hex(archive), testArchiveLinx))},
			wantUnsupported: true,
		},
		{
			name:            "checksum line for another file is unsupported",
			tag:             "v1.2.0",
			assets:          map[string][]byte{testArchiveLinx: archive, checksumsAssetName: otherSums},
			wantUnsupported: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newInstallLayout(t)
			srv := newFakeGitHub(t, releaseFixture{tag: tt.tag, prerelease: tt.prerelease, assets: tt.assets})
			inst := newTestInstaller(t, layout, srv.URL+"/release", nil)

			check, err := inst.Check(context.Background())

			if tt.wantUnsupported {
				require.ErrorIs(t, err, ErrAutoUpdateUnsupported)
				assert.False(t, check.Available)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantAvailable, check.Available)
			if tt.wantAvailable {
				assert.Equal(t, "1.2.0", check.Version)
			}
			assert.Equal(t, []string{"iworkflow"}, listing(t, layout.parent))
		})
	}
}

func TestArchiveInstallerCheckRefusesUnwritableParent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not POSIX on Windows")
	}
	archive := buildTarGz(t, standardEntries(testTop))
	layout := newInstallLayout(t)
	srv := newFakeGitHub(t, releaseFixture{tag: "v1.2.0", assets: releaseAssets(t, archive)})
	inst := newTestInstaller(t, layout, srv.URL+"/release", nil)
	require.NoError(t, os.Chmod(layout.parent, 0o555))
	t.Cleanup(func() { _ = os.Chmod(layout.parent, 0o755) })

	_, err := inst.Check(context.Background())

	require.ErrorIs(t, err, ErrAutoUpdateUnsupported)
}

func TestArchiveInstallerDownload(t *testing.T) {
	archive := buildTarGz(t, standardEntries(testTop))
	badSums := []byte(fmt.Sprintf("%s  %s\n", testZeroDigest, testArchiveLinx))
	tests := []struct {
		name      string
		assets    map[string][]byte
		maxBytes  int64
		wantError string
	}{
		{
			name:   "verified archive is kept in the install parent",
			assets: releaseAssets(t, archive),
		},
		{
			name:      "checksum mismatch removes the download",
			assets:    map[string][]byte{testArchiveLinx: archive, checksumsAssetName: badSums},
			wantError: "checksum mismatch",
		},
		{
			name:      "archive over the size cap is removed",
			assets:    releaseAssets(t, archive),
			maxBytes:  16,
			wantError: "larger than 16 bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newInstallLayout(t)
			srv := newFakeGitHub(t, releaseFixture{tag: "v1.2.0", assets: tt.assets})
			inst := newTestInstaller(t, layout, srv.URL+"/release", func(cfg *ArchiveConfig) {
				if tt.maxBytes > 0 {
					cfg.MaxDownloadBytes = tt.maxBytes
				}
			})
			check, err := inst.Check(context.Background())
			require.NoError(t, err)
			require.True(t, check.Available)

			err = inst.Download(context.Background(), nil)

			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				assert.Equal(t, []string{"iworkflow"}, listing(t, layout.parent))
				assertInstallUntouched(t, layout)
				return
			}
			require.NoError(t, err)
			downloads := 0
			for _, name := range listing(t, layout.parent) {
				if strings.HasPrefix(name, downloadPrefix) {
					downloads++
				}
			}
			assert.Equal(t, 1, downloads)
		})
	}
}

func TestArchiveInstallerDownloadRequiresACheck(t *testing.T) {
	layout := newInstallLayout(t)
	inst := newTestInstaller(t, layout, "http://127.0.0.1:1/release", nil)

	require.Error(t, inst.Download(context.Background(), nil))
}

func TestArchiveInstallerDownloadStopsWhenContextEnds(t *testing.T) {
	archive := buildTarGz(t, standardEntries(testTop))
	layout := newInstallLayout(t)
	srv := newFakeGitHub(t, releaseFixture{tag: "v1.2.0", assets: releaseAssets(t, archive), blockFile: testArchiveLinx})
	inst := newTestInstaller(t, layout, srv.URL+"/release", nil)
	check, err := inst.Check(context.Background())
	require.NoError(t, err)
	require.True(t, check.Available)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = inst.Download(ctx, nil)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, []string{"iworkflow"}, listing(t, layout.parent))
}

func TestArchiveInstallerQuitAndInstall(t *testing.T) {
	archive := buildTarGz(t, standardEntries(testTop))
	digest := sha256Hex(archive)
	tests := []struct {
		name     string
		startErr error
		wantExit bool
	}{
		{name: "helper started: app exits", wantExit: true},
		{name: "helper failed to start: install untouched", startErr: errInjected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newInstallLayout(t)
			srv := newFakeGitHub(t, releaseFixture{tag: "v1.2.0", assets: releaseAssets(t, archive)})
			var events []string
			var gotArgs HelperArgs
			inst := newTestInstaller(t, layout, srv.URL+"/release", func(cfg *ArchiveConfig) {
				cfg.StartHelper = func(args HelperArgs) error {
					events = append(events, "start")
					gotArgs = args
					return tt.startErr
				}
				cfg.Exit = func() { events = append(events, "exit") }
			})
			_, err := inst.Check(context.Background())
			require.NoError(t, err)
			require.NoError(t, inst.Download(context.Background(), nil))
			require.NoError(t, inst.Prepare(context.Background()))
			var onFailureErrs []error

			err = inst.QuitAndInstall(context.Background(), func(err error) { onFailureErrs = append(onFailureErrs, err) })

			require.NotEmpty(t, events)
			assert.Equal(t, "start", events[0])
			assert.Equal(t, digest, gotArgs.SHA256)
			assert.Equal(t, layout.target, gotArgs.Target)
			assert.Equal(t, layout.executable, gotArgs.Executable)
			assert.Equal(t, os.Getpid(), gotArgs.ParentPID)
			assert.Equal(t, layout.parent, filepath.Dir(gotArgs.Archive))
			assert.Equal(t, layout.parent, filepath.Dir(gotArgs.Staging))
			if tt.wantExit {
				require.NoError(t, err)
				assert.Equal(t, []string{"start", "exit"}, events)
				assert.Empty(t, onFailureErrs)
				// The app itself never swaps: the install is still the old one until the helper runs.
				assertOldContents(t, layout)
				return
			}
			require.ErrorIs(t, err, errInjected)
			assert.Equal(t, []string{"start"}, events)
			require.Len(t, onFailureErrs, 1)
			require.ErrorIs(t, onFailureErrs[0], errInjected)
			assertInstallUntouched(t, layout)
		})
	}
}

func TestApplyUpdateReplacesInstallAndKeepsPreviousUntilCleanup(t *testing.T) {
	layout := newInstallLayout(t)
	data := buildTarGz(t, standardEntries(testTop))
	archive := writeUpdateArchive(t, layout, data)
	var relaunched []string

	err := ApplyUpdate(context.Background(), applyOptions(layout, archive, sha256Hex(data), &relaunched))

	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(layout.target, "iworkflow-desktop"))
	require.NoError(t, err)
	assert.Equal(t, "new-binary", string(got))
	info, err := os.Stat(filepath.Join(layout.target, "iworkflow-desktop"))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	}
	server, err := os.ReadFile(filepath.Join(layout.target, "server", "server.bin"))
	require.NoError(t, err)
	assert.Equal(t, "new-server", string(server))
	assert.Equal(t, []string{filepath.Join(layout.target, "iworkflow-desktop")}, relaunched)
	assert.Equal(t, []string{"iworkflow", "iworkflow.old"}, listing(t, layout.parent))
	kept, err := os.ReadFile(filepath.Join(layout.parent, "iworkflow.old", "iworkflow-desktop"))
	require.NoError(t, err)
	assert.Equal(t, "old-binary", string(kept))

	require.NoError(t, CleanupPreviousInstall(layout.target))
	assert.Equal(t, []string{"iworkflow"}, listing(t, layout.parent))
	require.NoError(t, CleanupPreviousInstall(layout.target))
}

func TestApplyUpdateRollsBackFailedSwap(t *testing.T) {
	tests := []struct {
		name     string
		failCall int
	}{
		{name: "first move aside fails", failCall: 1},
		{name: "move into place fails and the previous install is restored", failCall: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newInstallLayout(t)
			archive := writeUpdateArchive(t, layout, buildTarGz(t, standardEntries(testTop)))
			var relaunched []string
			calls := 0
			opts := applyOptions(layout, archive, sha256Hex(buildTarGz(t, standardEntries(testTop))), &relaunched)
			opts.Rename = func(oldpath, newpath string) error {
				calls++
				if calls == tt.failCall {
					return errInjected
				}
				return os.Rename(oldpath, newpath)
			}

			err := ApplyUpdate(context.Background(), opts)

			require.ErrorIs(t, err, errInjected)
			assertInstallUntouched(t, layout)
			assert.Empty(t, relaunched)
		})
	}
}

func TestApplyUpdateLeavesInstallUntouchedWhenParentStaysAlive(t *testing.T) {
	layout := newInstallLayout(t)
	archive := writeUpdateArchive(t, layout, buildTarGz(t, standardEntries(testTop)))
	var relaunched []string
	opts := applyOptions(layout, archive, sha256Hex(buildTarGz(t, standardEntries(testTop))), &relaunched)
	opts.WaitTimeout = 20 * time.Millisecond
	opts.WaitForExit = func(ctx context.Context, _ int) error {
		<-ctx.Done()
		return ctx.Err()
	}

	err := ApplyUpdate(context.Background(), opts)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assertInstallUntouched(t, layout)
	assert.Empty(t, relaunched)
}

func TestApplyUpdateRejectsBadArchivesAndLeavesInstallUntouched(t *testing.T) {
	tests := []struct {
		name     string
		entries  []tarEntry
		digest   string
		maxBytes int64
	}{
		{
			name:    "checksum mismatch",
			entries: standardEntries(testTop),
			digest:  testZeroDigest,
		},
		{
			name: "parent reference escapes the staging folder",
			entries: append(standardEntries(testTop), tarEntry{
				name: testTop + "/../../evil", typeflag: tar.TypeReg, mode: 0o644, body: "evil",
			}),
		},
		{
			name: "absolute entry name",
			entries: append(standardEntries(testTop), tarEntry{
				name: "/tmp/evil", typeflag: tar.TypeReg, mode: 0o644, body: "evil",
			}),
		},
		{
			name: "symlink escaping the staging folder",
			entries: append(standardEntries(testTop), tarEntry{
				name: testTop + "/link", typeflag: tar.TypeSymlink, linkname: "../../../outside",
			}),
		},
		{
			name: "hard link escaping the staging folder",
			entries: append(standardEntries(testTop), tarEntry{
				name: testTop + "/hard", typeflag: tar.TypeLink, linkname: testTop + "/../../../etc/passwd",
			}),
		},
		{
			name: "two top folders",
			entries: append(standardEntries(testTop), tarEntry{
				name: "other/file", typeflag: tar.TypeReg, mode: 0o644, body: "x",
			}),
		},
		{
			name: "archive without the executable",
			entries: []tarEntry{
				{name: testTop + "/", typeflag: tar.TypeDir, mode: 0o755},
				{name: testTop + "/server/server.bin", typeflag: tar.TypeReg, mode: 0o644, body: "new-server"},
			},
		},
		{
			name:     "extracted size over the cap",
			entries:  standardEntries(testTop),
			maxBytes: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newInstallLayout(t)
			data := buildTarGz(t, tt.entries)
			archive := writeUpdateArchive(t, layout, data)
			digest := tt.digest
			if digest == "" {
				digest = sha256Hex(data)
			}
			var relaunched []string
			opts := applyOptions(layout, archive, digest, &relaunched)
			if tt.maxBytes > 0 {
				opts.MaxExtractBytes = tt.maxBytes
			}

			err := ApplyUpdate(context.Background(), opts)

			require.Error(t, err)
			assertInstallUntouched(t, layout)
			assert.Empty(t, relaunched)
		})
	}
}

func TestApplyUpdateRefusesArchiveOutsideTheInstallParent(t *testing.T) {
	layout := newInstallLayout(t)
	data := buildTarGz(t, standardEntries(testTop))
	outside := filepath.Join(t.TempDir(), ".iworkflow-update-test.tar.gz")
	require.NoError(t, os.WriteFile(outside, data, 0o600))
	var relaunched []string

	err := ApplyUpdate(context.Background(), applyOptions(layout, outside, sha256Hex(data), &relaunched))

	require.Error(t, err)
	assert.FileExists(t, outside, "a file the helper did not validate must not be deleted")
	assertInstallUntouched(t, layout)
}

func TestNewArchiveInstallerValidatesConfig(t *testing.T) {
	layout := newInstallLayout(t)
	valid := ArchiveConfig{
		AppVersion:  "1.1.0",
		Target:      layout.target,
		Executable:  layout.executable,
		StartHelper: func(HelperArgs) error { return nil },
		Exit:        func() {},
	}
	tests := []struct {
		name   string
		mutate func(*ArchiveConfig)
	}{
		{name: "missing version", mutate: func(c *ArchiveConfig) { c.AppVersion = "" }},
		{name: "relative target", mutate: func(c *ArchiveConfig) { c.Target = "iworkflow" }},
		{name: "executable outside target", mutate: func(c *ArchiveConfig) { c.Executable = filepath.Join(layout.parent, "other") }},
		{name: "missing StartHelper", mutate: func(c *ArchiveConfig) { c.StartHelper = nil }},
		{name: "missing Exit", mutate: func(c *ArchiveConfig) { c.Exit = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)

			_, err := NewArchiveInstaller(cfg)

			require.Error(t, err)
		})
	}
}

func TestCleanupPreviousInstall(t *testing.T) {
	tests := []struct {
		name       string
		withBackup bool
	}{
		{name: "nothing to clean", withBackup: false},
		{name: "previous install is removed", withBackup: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newInstallLayout(t)
			if tt.withBackup {
				require.NoError(t, os.Mkdir(layout.target+".old", 0o755))
			}

			require.NoError(t, CleanupPreviousInstall(layout.target))

			assert.Equal(t, []string{"iworkflow"}, listing(t, layout.parent))
		})
	}
}

func TestInstallTargetFromExecutable(t *testing.T) {
	tests := []struct {
		name       string
		executable string
		goos       string
		want       string
		wantErr    bool
	}{
		{name: "linux folder of the executable", executable: "/opt/iworkflow/iworkflow-desktop", goos: "linux", want: "/opt/iworkflow"},
		{name: "darwin bundle root", executable: "/Applications/iWorkflow.app/Contents/MacOS/iworkflow", goos: "darwin", want: "/Applications/iWorkflow.app"},
		{name: "darwin outside a bundle", executable: "/opt/iworkflow/iworkflow", goos: "darwin", wantErr: true},
		{name: "relative executable", executable: "iworkflow-desktop", goos: "linux", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && !tt.wantErr {
				t.Skip("the fixture uses POSIX absolute paths")
			}
			got, err := InstallTargetFromExecutable(tt.executable, tt.goos)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, filepath.FromSlash(tt.want), got)
		})
	}
}

func TestParseChecksumsAcceptsBinaryMarkerAndRejectsMalformedLines(t *testing.T) {
	good := strings.Repeat("ab", 32)
	tests := []struct {
		name    string
		text    string
		want    map[string]string
		wantErr bool
	}{
		{name: "two spaces", text: good + "  a.tar.gz\n", want: map[string]string{"a.tar.gz": good}},
		{name: "binary marker", text: good + " *a.tar.gz\n", want: map[string]string{"a.tar.gz": good}},
		{name: "short digest", text: "abcd  a.tar.gz\n", wantErr: true},
		{name: "one field", text: good + "\n", wantErr: true},
		{name: "duplicate name", text: good + "  a.tar.gz\n" + good + "  a.tar.gz\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChecksums(tt.text)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestArchiveInstallerDownloadReportsProgressUpToTheArchiveSize(t *testing.T) {
	archive := buildTarGz(t, standardEntries(testTop))
	layout := newInstallLayout(t)
	srv := newFakeGitHub(t, releaseFixture{tag: "v1.2.0", assets: releaseAssets(t, archive)})
	inst := newTestInstaller(t, layout, srv.URL+"/release", nil)
	_, err := inst.Check(context.Background())
	require.NoError(t, err)

	var done, total int64
	require.NoError(t, inst.Download(context.Background(), func(d int64, tot int64) { done, total = d, tot }))

	assert.Equal(t, int64(len(archive)), done)
	assert.Equal(t, int64(len(archive)), total)
}

func TestApplyUpdateUsesAnAlreadyExtractedStagingFolder(t *testing.T) {
	layout := newInstallLayout(t)
	data := buildTarGz(t, standardEntries(testTop))
	archive := writeUpdateArchive(t, layout, data)
	staging, err := extractToStaging(ApplyOptions{Archive: archive, Target: layout.target, MaxExtractBytes: DefaultMaxExtractBytes, Log: func(string) {}}, "iworkflow-desktop")
	require.NoError(t, err)
	var relaunched []string
	opts := applyOptions(layout, archive, sha256Hex(data), &relaunched)
	opts.Staging = staging

	require.NoError(t, ApplyUpdate(context.Background(), opts))

	got, err := os.ReadFile(filepath.Join(layout.target, "iworkflow-desktop"))
	require.NoError(t, err)
	assert.Equal(t, "new-binary", string(got))
	assert.Len(t, relaunched, 1)
}

func TestApplyUpdateRejectsAStagingFolderOutsideTheInstallParent(t *testing.T) {
	layout := newInstallLayout(t)
	data := buildTarGz(t, standardEntries(testTop))
	archive := writeUpdateArchive(t, layout, data)
	var relaunched []string
	opts := applyOptions(layout, archive, sha256Hex(data), &relaunched)
	opts.Staging = filepath.Join(t.TempDir(), ".iworkflow-staging-x")

	require.ErrorContains(t, ApplyUpdate(context.Background(), opts), "staging folder")

	assertOldContents(t, layout)
}

func TestCleanupStaleUpdateFilesRemovesOnlyOldUpdateLeftovers(t *testing.T) {
	layout := newInstallLayout(t)
	now := time.Now()
	old := now.Add(-2 * time.Hour)
	mk := func(name string, dir bool, modTime time.Time) {
		path := filepath.Join(layout.parent, name)
		if dir {
			require.NoError(t, os.Mkdir(path, 0o750))
			require.NoError(t, os.WriteFile(filepath.Join(path, "file"), []byte("x"), 0o600))
		} else {
			require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
		}
		require.NoError(t, os.Chtimes(path, modTime, modTime))
	}
	mk(".iworkflow-staging-old", true, old)
	mk(".iworkflow-update-old.tar.gz", false, old)
	mk(".iworkflow-staging-fresh", true, now)
	mk("unrelated", true, old)

	require.NoError(t, CleanupStaleUpdateFiles(layout.target, now, StaleUpdateMaxAge))

	assert.Equal(t, []string{".iworkflow-staging-fresh", "iworkflow", "unrelated"}, listing(t, layout.parent))
}

func TestRetryRenameSucceedsAfterTransientFailures(t *testing.T) {
	calls, sleeps := 0, 0
	rename := func(string, string) error {
		calls++
		if calls < 4 {
			return errInjected
		}
		return nil
	}

	err := retryRename(rename, func(time.Duration) { sleeps++ }, "a", "b")

	require.NoError(t, err)
	assert.Equal(t, 4, calls)
	assert.Equal(t, 3, sleeps)
}

func TestRetryRenameStopsAtOnceWhenTheFolderIsMissing(t *testing.T) {
	calls, sleeps := 0, 0
	rename := func(string, string) error { calls++; return fs.ErrNotExist }

	err := retryRename(rename, func(time.Duration) { sleeps++ }, "a", "b")

	require.ErrorIs(t, err, fs.ErrNotExist)
	assert.Equal(t, 1, calls)
	assert.Zero(t, sleeps)
}

func TestRetryRenameGivesUpAfterTheLastAttemptWithoutSleepingAfterIt(t *testing.T) {
	calls, sleeps := 0, 0
	rename := func(string, string) error { calls++; return errInjected }

	err := retryRename(rename, func(time.Duration) { sleeps++ }, "a", "b")

	require.ErrorIs(t, err, errInjected)
	assert.Equal(t, renameAttempts, calls)
	assert.Equal(t, renameAttempts-1, sleeps)
}
