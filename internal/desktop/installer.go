package desktop

// The archive installer of the auto strategy. The app side (Check, Download, QuitAndInstall) reads the
// GitHub release, downloads and verifies the archive, then starts the updater helper and ends. The helper
// side (ApplyUpdate, run by cmd/iworkflow-updater) waits for the app to exit, extracts the verified archive
// next to the install folder and swaps the folders with rollback. Archive extraction lives in
// installer_extract.go.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// DefaultArchiveHTTPTimeout bounds one archive or checksum transfer.
	DefaultArchiveHTTPTimeout = 15 * time.Minute
	// DefaultMaxDownloadBytes caps the downloaded archive size (600 MiB).
	DefaultMaxDownloadBytes int64 = 600 << 20
	// DefaultMaxExtractBytes caps the total size of the regular files extracted by the helper (2 GiB).
	DefaultMaxExtractBytes int64 = 2 << 30
	// DefaultHelperWaitTimeout bounds how long the helper waits for the app to exit.
	DefaultHelperWaitTimeout = 2 * time.Minute

	checksumsAssetName = "SHA256SUMS"
	maxChecksumsBytes  = 64 << 10
	backupSuffix       = ".old"
	stagingPattern     = ".iworkflow-staging-*"
	downloadPrefix     = ".iworkflow-update-"
	downloadPattern    = downloadPrefix + "*.tar.gz"
	probePattern       = ".iworkflow-probe-*"
	exitPollInterval   = 200 * time.Millisecond
	sha256HexLength    = 64
)

var _ Installer = (*ArchiveInstaller)(nil)

// ArchiveConfig configures an ArchiveInstaller. AppVersion, Target, Executable, StartHelper and Exit are
// required; the other fields have defaults.
type ArchiveConfig struct {
	// Repo is the "owner/name" GitHub repository. Empty means UpdateRepo.
	Repo string
	// APIURL overrides the latest-release URL. Empty means LatestReleaseAPIURL(Repo).
	APIURL string
	// AppVersion is the running version without a leading v, for example "0.16.0".
	AppVersion string
	// GOOS and GOARCH select the archive asset. Empty means the running platform.
	GOOS   string
	GOARCH string
	// GHToken, when set, is sent as a Bearer token. Never logged.
	GHToken string
	// Target is the absolute install folder that an update replaces: the folder of the executable, or the
	// .app bundle root on macOS (see InstallTargetFromExecutable).
	Target string
	// Executable is the absolute path of the running executable. It must be inside Target.
	Executable string
	// HTTPClient performs the API and download requests. Nil means a client with DefaultArchiveHTTPTimeout.
	HTTPClient *http.Client
	// MaxDownloadBytes caps the archive size. Zero means DefaultMaxDownloadBytes.
	MaxDownloadBytes int64
	// StartHelper starts the detached updater helper with args. It must not return before the helper
	// process has started: the app ends right after it returns, and a helper that did not start would leave
	// the user without an app. A non-nil error means nothing was started and the install is untouched.
	StartHelper func(args HelperArgs) error
	// Exit ends the app once StartHelper succeeded, so the helper can replace the files.
	Exit func()
	// Log receives diagnostic messages. Optional.
	Log func(message string)
}

// HelperArgs are the arguments the app passes to the updater helper.
type HelperArgs struct {
	// Archive is the verified tar.gz downloaded next to Target. The helper deletes it.
	Archive string
	// SHA256 is the expected lowercase hex digest of Archive. The helper verifies it before extracting.
	SHA256 string
	// Target is the install folder to replace.
	Target string
	// ParentPID is the process ID of the app, which the helper waits for before touching anything.
	ParentPID int
	// Executable is the running executable inside Target. The helper relaunches the same relative path.
	Executable string
	// Staging is the folder the app already extracted the update into, next to Target. The helper only swaps
	// it into place, so the slow extraction happens in the app, where the user sees it.
	Staging string
}

// ArchiveInstaller implements Installer from the GitHub release archives. Check and Download run in the
// app; the file swap runs in the helper (ApplyUpdate). It is safe for concurrent use.
type ArchiveInstaller struct {
	cfg     ArchiveConfig
	mu      sync.Mutex
	pending *pendingUpdate
}

// pendingUpdate is the update found by the last Check, and the downloaded archive once Download succeeded.
type pendingUpdate struct {
	version     string
	archiveName string
	archiveURL  string
	sha256      string
	archivePath string
	staging     string
}

// releaseAsset is the subset of a GitHub release asset that is read.
type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// releaseDocument is the subset of the GitHub "latest release" response that the archive installer reads.
type releaseDocument struct {
	TagName    string         `json:"tag_name"`
	HTMLURL    string         `json:"html_url"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

// NewArchiveInstaller validates cfg, applies defaults and returns an ArchiveInstaller.
func NewArchiveInstaller(cfg ArchiveConfig) (*ArchiveInstaller, error) {
	if cfg.AppVersion == "" {
		return nil, errors.New("archive installer needs the application version")
	}
	if _, err := executableRel(cfg.Target, cfg.Executable); err != nil {
		return nil, fmt.Errorf("archive installer: %w", err)
	}
	if cfg.StartHelper == nil {
		return nil, errors.New("archive installer needs a StartHelper function")
	}
	if cfg.Exit == nil {
		return nil, errors.New("archive installer needs an Exit function")
	}
	if cfg.Repo == "" {
		cfg.Repo = UpdateRepo
	}
	if cfg.APIURL == "" {
		cfg.APIURL = LatestReleaseAPIURL(cfg.Repo)
	}
	if cfg.GOOS == "" {
		cfg.GOOS = runtime.GOOS
	}
	if cfg.GOARCH == "" {
		cfg.GOARCH = runtime.GOARCH
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: DefaultArchiveHTTPTimeout}
	}
	if cfg.MaxDownloadBytes <= 0 {
		cfg.MaxDownloadBytes = DefaultMaxDownloadBytes
	}
	return &ArchiveInstaller{cfg: cfg}, nil
}

// Check reports Available only when a newer release has both the archive for this platform and a SHA256SUMS
// line for it, and when the install folder can be replaced. Otherwise it returns an error wrapping
// ErrAutoUpdateUnsupported, so the flow falls back to the notice. A release that is not newer is not an error.
func (a *ArchiveInstaller) Check(ctx context.Context) (UpdateCheck, error) {
	doc, err := a.fetchRelease(ctx)
	if err != nil {
		return UpdateCheck{}, err
	}
	notice, ok := ParseLatestRelease(&ReleasePayload{
		TagName:    doc.TagName,
		HTMLURL:    doc.HTMLURL,
		Draft:      doc.Draft,
		Prerelease: doc.Prerelease,
	})
	if !ok || !IsNewerVersion(a.cfg.AppVersion, notice.Version) {
		return UpdateCheck{}, nil
	}
	version := notice.Version
	archiveName := archiveAssetName(version, a.cfg.GOOS, a.cfg.GOARCH)
	archive, ok := findAsset(doc.Assets, archiveName)
	if !ok {
		return UpdateCheck{}, fmt.Errorf("release %s has no %s: %w", version, archiveName, ErrAutoUpdateUnsupported)
	}
	sums, ok := findAsset(doc.Assets, checksumsAssetName)
	if !ok {
		return UpdateCheck{}, fmt.Errorf("release %s has no %s: %w", version, checksumsAssetName, ErrAutoUpdateUnsupported)
	}
	digests, err := a.fetchChecksums(ctx, sums.BrowserDownloadURL)
	if err != nil {
		return UpdateCheck{}, err
	}
	digest, ok := digests[archiveName]
	if !ok {
		return UpdateCheck{}, fmt.Errorf("%s lists no checksum for %s: %w", checksumsAssetName, archiveName, ErrAutoUpdateUnsupported)
	}
	if err := a.checkInstallLocation(); err != nil {
		return UpdateCheck{}, err
	}
	a.mu.Lock()
	previous := a.pending
	a.pending = &pendingUpdate{
		version:     version,
		archiveName: archiveName,
		archiveURL:  archive.BrowserDownloadURL,
		sha256:      digest,
	}
	a.mu.Unlock()
	if previous != nil && previous.archivePath != "" {
		a.removeFile(previous.archivePath)
	}
	return UpdateCheck{Available: true, Version: version}, nil
}

// Download fetches the archive found by the last Check into the install parent folder and verifies its
// SHA-256 against SHA256SUMS. A failed or oversized download, or a checksum mismatch, removes the file.
func (a *ArchiveInstaller) Download(ctx context.Context, progress func(done int64, total int64)) error {
	a.mu.Lock()
	pending := a.pending
	a.mu.Unlock()
	if pending == nil {
		return errors.New("no update to download: Check must find an available update first")
	}
	req, err := a.newRequest(ctx, pending.archiveURL, "application/octet-stream")
	if err != nil {
		return err
	}
	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", pending.archiveName, err)
	}
	defer a.closeBody(resp.Body)
	if err := checkStatus(resp); err != nil {
		return fmt.Errorf("download %s: %w", pending.archiveName, err)
	}

	file, err := os.CreateTemp(filepath.Dir(a.cfg.Target), downloadPattern)
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	path := file.Name()
	digest := sha256.New()
	sinks := []io.Writer{file, digest}
	if progress != nil {
		sinks = append(sinks, &progressWriter{total: resp.ContentLength, report: progress})
	}
	written, copyErr := io.Copy(io.MultiWriter(sinks...), io.LimitReader(resp.Body, a.cfg.MaxDownloadBytes+1))
	closeErr := file.Close()
	switch {
	case copyErr != nil:
		err = fmt.Errorf("download %s: %w", pending.archiveName, copyErr)
	case closeErr != nil:
		err = fmt.Errorf("close download file: %w", closeErr)
	case written > a.cfg.MaxDownloadBytes:
		err = fmt.Errorf("download %s is larger than %d bytes", pending.archiveName, a.cfg.MaxDownloadBytes)
	case hex.EncodeToString(digest.Sum(nil)) != pending.sha256:
		err = fmt.Errorf("checksum mismatch for %s", pending.archiveName)
	}
	if err != nil {
		a.removeFile(path)
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending != pending {
		a.removeFile(path)
		return errors.New("update changed while it was downloading")
	}
	previous := pending.archivePath
	pending.archivePath = path
	if previous != "" {
		a.removeFile(previous)
	}
	return nil
}

// progressWriter reports the bytes written so far. total is -1 when the server did not announce a size.
type progressWriter struct {
	done   int64
	total  int64
	report func(done int64, total int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.done += int64(len(p))
	w.report(w.done, w.total)
	return len(p), nil
}

// Prepare extracts the downloaded archive into a staging folder next to the install, while the app is still
// running, so the slow part of the install is visible to the user and a failure leaves the app untouched.
func (a *ArchiveInstaller) Prepare(ctx context.Context) error {
	a.mu.Lock()
	pending := a.pending
	var archive string
	if pending != nil {
		archive = pending.archivePath
	}
	a.mu.Unlock()
	if archive == "" {
		return errors.New("no downloaded update to prepare")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("prepare update: %w", err)
	}
	rel, err := executableRel(a.cfg.Target, a.cfg.Executable)
	if err != nil {
		return err
	}
	staging, err := extractToStaging(ApplyOptions{
		Archive:         archive,
		Target:          a.cfg.Target,
		MaxExtractBytes: DefaultMaxExtractBytes,
		Log:             a.log,
	}, rel)
	if err != nil {
		a.log(fmt.Sprintf("[update] prepare: %v", err))
		a.discard(pending)
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending != pending {
		removeAllLogged(staging, a.log)
		return errors.New("update changed while it was prepared")
	}
	pending.staging = staging
	return nil
}

// QuitAndInstall hands the verified archive to the updater helper and then calls Exit, so the app ends and
// the helper can replace the files. It never touches the install folder itself. If StartHelper fails, the
// archive is removed, the install is left untouched, onFailure is called and the error is returned.
func (a *ArchiveInstaller) QuitAndInstall(ctx context.Context, onFailure func(error)) error {
	a.mu.Lock()
	pending := a.pending
	var archive, digest, staging string
	if pending != nil {
		archive, digest, staging = pending.archivePath, pending.sha256, pending.staging
	}
	a.mu.Unlock()
	if archive == "" || staging == "" {
		return errors.New("no prepared update to install")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("install update: %w", err)
	}
	args := HelperArgs{
		Staging:    staging,
		Archive:    archive,
		SHA256:     digest,
		Target:     a.cfg.Target,
		ParentPID:  os.Getpid(),
		Executable: a.cfg.Executable,
	}
	if err := a.cfg.StartHelper(args); err != nil {
		return a.failInstall(pending, onFailure, fmt.Errorf("start update helper: %w", err))
	}
	a.cfg.Exit()
	return nil
}

// failInstall drops the pending update, reports err through onFailure and returns it.
func (a *ArchiveInstaller) failInstall(pending *pendingUpdate, onFailure func(error), err error) error {
	a.log(fmt.Sprintf("[update] %v", err))
	a.discard(pending)
	if onFailure != nil {
		onFailure(err)
	}
	return err
}

// ApplyOptions are the inputs of ApplyUpdate, run by the helper process.
type ApplyOptions struct {
	// Archive is the downloaded tar.gz. It must sit in the parent folder of Target and be named like
	// the archives Download creates. It is deleted at the end in every case, once it has passed validation.
	Archive string
	// SHA256 is the expected lowercase hex digest of Archive. Required.
	SHA256 string
	// Target is the absolute install folder to replace.
	Target string
	// Executable is the running executable inside the old Target. The relaunch path is the same relative
	// path inside the new Target.
	Executable string
	// ParentPID is the app process that must exit before anything is touched.
	ParentPID int
	// Staging is an already extracted update in the parent folder of Target. When set, ApplyUpdate skips the
	// extraction, and removes the folder if the update does not get installed. Optional.
	Staging string
	// WaitTimeout bounds the wait for ParentPID. Zero means DefaultHelperWaitTimeout.
	WaitTimeout time.Duration
	// MaxExtractBytes caps the size of the extracted regular files. Zero means DefaultMaxExtractBytes.
	MaxExtractBytes int64
	// Relaunch starts the new executable. Required. It must return once the new process has started.
	Relaunch func(exe string) error
	// Log receives diagnostic messages. Optional.
	Log func(message string)
	// WaitForExit blocks until pid has exited or ctx ends. Nil means waitForProcessExit.
	WaitForExit func(ctx context.Context, pid int) error
	// Rename moves folders. Nil means os.Rename. Tests inject failures here.
	Rename func(oldpath, newpath string) error
}

// ApplyUpdate runs in the updater helper after the app quit. It waits for ParentPID, verifies the archive,
// extracts it into a staging folder next to Target, moves Target to Target+".old" and the staging folder to
// Target, then relaunches the executable inside the new Target. A failed move is rolled back. A failed
// relaunch keeps the new install and the ".old" copy. The previous install is removed later by
// CleanupPreviousInstall, which the helper calls after a successful relaunch. ApplyUpdate never ends the
// process itself.
func ApplyUpdate(ctx context.Context, opts ApplyOptions) error {
	opts, rel, err := opts.normalized()
	if err != nil {
		return err
	}
	// Validation passed, so the archive is ours: it is removed in every case from here on.
	defer removeLogged(opts.Archive, opts.Log)
	if opts.Staging != "" {
		// After a successful swap this path no longer exists, so the removal only matters on failure.
		defer removeAllLogged(opts.Staging, opts.Log)
	}

	waitCtx, cancel := context.WithTimeout(ctx, opts.WaitTimeout)
	defer cancel()
	if err := opts.WaitForExit(waitCtx, opts.ParentPID); err != nil {
		return fmt.Errorf("wait for the app (pid %d) to exit: %w", opts.ParentPID, err)
	}
	if err := verifyFileSHA256(opts.Archive, opts.SHA256); err != nil {
		return err
	}
	staging := opts.Staging
	if staging == "" {
		staging, err = extractToStaging(opts, rel)
		if err != nil {
			return err
		}
	}
	if err := swapFolders(staging, opts); err != nil {
		removeAllLogged(staging, opts.Log)
		return fmt.Errorf("swap install folders: %w", err)
	}
	newExe := filepath.Join(opts.Target, rel)
	if err := opts.Relaunch(newExe); err != nil {
		return fmt.Errorf("relaunch %s (new install kept, previous copy at %s): %w", newExe, opts.Target+backupSuffix, err)
	}
	return nil
}

// CleanupPreviousInstall removes the previous install that ApplyUpdate kept at target+".old". It does nothing
// when there is none. The shell calls it at start, and the helper calls it after a successful relaunch.
func CleanupPreviousInstall(target string) error {
	backup := filepath.Clean(target) + backupSuffix
	if _, err := os.Lstat(backup); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect previous install %s: %w", backup, err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous install %s: %w", backup, err)
	}
	return nil
}

// InstallTargetFromExecutable returns the folder that an update replaces for the running executable: the
// .app bundle root on darwin, and the folder of the executable elsewhere.
func InstallTargetFromExecutable(executable string, goos string) (string, error) {
	if goos == "darwin" {
		return MacOSBundleRoot(executable)
	}
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("executable path %q must be absolute", executable)
	}
	return filepath.Dir(executable), nil
}

// MacOSBundleRoot returns the .app bundle root of an executable laid out as <Name>.app/Contents/MacOS/<bin>.
func MacOSBundleRoot(executable string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("executable path %q must be absolute", executable)
	}
	macOSDir := filepath.Dir(executable)
	contentsDir := filepath.Dir(macOSDir)
	bundle := filepath.Dir(contentsDir)
	if filepath.Base(macOSDir) != "MacOS" || filepath.Base(contentsDir) != "Contents" || !strings.HasSuffix(filepath.Base(bundle), ".app") {
		return "", fmt.Errorf("executable %s is not inside an app bundle", executable)
	}
	return bundle, nil
}

// fetchRelease reads the latest release document from the GitHub API.
func (a *ArchiveInstaller) fetchRelease(ctx context.Context) (releaseDocument, error) {
	var doc releaseDocument
	reqCtx, cancel := context.WithTimeout(ctx, APIRequestTimeout)
	defer cancel()
	req, err := a.newRequest(reqCtx, a.cfg.APIURL, "application/vnd.github+json")
	if err != nil {
		return doc, err
	}
	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return doc, fmt.Errorf("fetch latest release: %w", err)
	}
	defer a.closeBody(resp.Body)
	if err := checkStatus(resp); err != nil {
		return doc, err
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseBodySize)).Decode(&doc); err != nil {
		return doc, fmt.Errorf("decode latest release: %w", err)
	}
	return doc, nil
}

// fetchChecksums downloads SHA256SUMS and parses it.
func (a *ArchiveInstaller) fetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, APIRequestTimeout)
	defer cancel()
	req, err := a.newRequest(reqCtx, url, "application/octet-stream")
	if err != nil {
		return nil, err
	}
	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", checksumsAssetName, err)
	}
	defer a.closeBody(resp.Body)
	if err := checkStatus(resp); err != nil {
		return nil, fmt.Errorf("fetch %s: %w", checksumsAssetName, err)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumsBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", checksumsAssetName, err)
	}
	if len(body) > maxChecksumsBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", checksumsAssetName, maxChecksumsBytes)
	}
	return parseChecksums(string(body))
}

// parseChecksums reads "<sha256>  <file>" lines. A leading "*" on the file name (binary mode) is accepted.
func parseChecksums(text string) (map[string]string, error) {
	digests := make(map[string]string)
	for index, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s line %d: want \"<sha256>  <file>\"", checksumsAssetName, index+1)
		}
		digest := strings.ToLower(fields[0])
		if !isSHA256Hex(digest) {
			return nil, fmt.Errorf("%s line %d: invalid sha256 digest", checksumsAssetName, index+1)
		}
		name := strings.TrimPrefix(fields[1], "*")
		if _, duplicate := digests[name]; duplicate {
			return nil, fmt.Errorf("%s lists %s twice", checksumsAssetName, name)
		}
		digests[name] = digest
	}
	return digests, nil
}

// checkInstallLocation reports ErrAutoUpdateUnsupported when the install folder is missing or its parent
// cannot take new entries. The probe is a temporary file in the parent, removed at once.
func (a *ArchiveInstaller) checkInstallLocation() error {
	info, err := os.Stat(a.cfg.Target)
	if err != nil {
		return fmt.Errorf("install folder %s: %w: %w", a.cfg.Target, err, ErrAutoUpdateUnsupported)
	}
	if !info.IsDir() {
		return fmt.Errorf("install folder %s is not a directory: %w", a.cfg.Target, ErrAutoUpdateUnsupported)
	}
	parent := filepath.Dir(a.cfg.Target)
	probe, err := os.CreateTemp(parent, probePattern)
	if err != nil {
		return fmt.Errorf("install folder parent %s is not writable (%w): %w", parent, err, ErrAutoUpdateUnsupported)
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close probe %s: %w", name, err)
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("remove probe %s: %w", name, err)
	}
	return nil
}

// discard forgets pending and removes its downloaded archive.
func (a *ArchiveInstaller) discard(pending *pendingUpdate) {
	if pending == nil {
		return
	}
	a.mu.Lock()
	if a.pending == pending {
		a.pending = nil
	}
	path, staging := pending.archivePath, pending.staging
	pending.archivePath, pending.staging = "", ""
	a.mu.Unlock()
	if path != "" {
		a.removeFile(path)
	}
	if staging != "" {
		removeAllLogged(staging, a.log)
	}
}

func (a *ArchiveInstaller) newRequest(ctx context.Context, url string, accept string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", userAgent)
	if a.cfg.GHToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.GHToken)
	}
	return req, nil
}

func (a *ArchiveInstaller) removeFile(path string) {
	removeLogged(path, a.log)
}

func (a *ArchiveInstaller) closeBody(body io.Closer) {
	if err := body.Close(); err != nil {
		a.log(fmt.Sprintf("[update] close response: %v", err))
	}
}

func (a *ArchiveInstaller) log(message string) {
	if a.cfg.Log != nil {
		a.cfg.Log(message)
	}
}

// normalized fills the defaults of opts and validates it. Nothing is touched on failure.
func (opts ApplyOptions) normalized() (ApplyOptions, string, error) {
	if !filepath.IsAbs(opts.Target) {
		return opts, "", fmt.Errorf("target %q must be absolute", opts.Target)
	}
	rel, err := executableRel(opts.Target, opts.Executable)
	if err != nil {
		return opts, "", err
	}
	if !filepath.IsAbs(opts.Archive) || filepath.Dir(opts.Archive) != filepath.Dir(opts.Target) {
		return opts, "", fmt.Errorf("archive %q must sit in the parent folder of the install target", opts.Archive)
	}
	base := filepath.Base(opts.Archive)
	if !strings.HasPrefix(base, downloadPrefix) || !strings.HasSuffix(base, ".tar.gz") {
		return opts, "", fmt.Errorf("archive %q is not an update download", opts.Archive)
	}
	if !isSHA256Hex(strings.ToLower(opts.SHA256)) {
		return opts, "", errors.New("apply update needs the expected sha256 of the archive")
	}
	opts.SHA256 = strings.ToLower(opts.SHA256)
	if opts.Staging != "" {
		matched, matchErr := filepath.Match(stagingPattern, filepath.Base(opts.Staging))
		if !filepath.IsAbs(opts.Staging) || filepath.Dir(opts.Staging) != filepath.Dir(opts.Target) || matchErr != nil || !matched {
			return opts, "", fmt.Errorf("staging folder %q must be a staging folder next to the install target", opts.Staging)
		}
	}
	if opts.ParentPID <= 0 {
		return opts, "", fmt.Errorf("apply update needs the parent process id, got %d", opts.ParentPID)
	}
	if opts.Relaunch == nil {
		return opts, "", errors.New("apply update needs a Relaunch function")
	}
	if opts.WaitTimeout <= 0 {
		opts.WaitTimeout = DefaultHelperWaitTimeout
	}
	if opts.MaxExtractBytes <= 0 {
		opts.MaxExtractBytes = DefaultMaxExtractBytes
	}
	if opts.WaitForExit == nil {
		opts.WaitForExit = waitForProcessExit
	}
	if opts.Rename == nil {
		opts.Rename = renameWithRetry
	}
	if opts.Log == nil {
		opts.Log = func(string) {}
	}
	return opts, rel, nil
}

// extractToStaging extracts the archive into a new folder next to Target and checks that the executable is
// in it. The staging folder gets the mode of Target, because MkdirTemp creates it private.
func extractToStaging(opts ApplyOptions, rel string) (string, error) {
	staging, err := os.MkdirTemp(filepath.Dir(opts.Target), stagingPattern)
	if err != nil {
		return "", fmt.Errorf("create staging folder: %w", err)
	}
	fail := func(err error) (string, error) {
		removeAllLogged(staging, opts.Log)
		return "", err
	}
	info, err := os.Stat(opts.Target)
	if err != nil {
		return fail(fmt.Errorf("inspect install folder: %w", err))
	}
	if err := os.Chmod(staging, info.Mode().Perm()); err != nil {
		return fail(fmt.Errorf("set staging folder mode: %w", err))
	}
	file, err := os.Open(opts.Archive)
	if err != nil {
		return fail(fmt.Errorf("open archive: %w", err))
	}
	extractErr := extractArchive(file, staging, opts.MaxExtractBytes)
	closeErr := file.Close()
	if extractErr != nil {
		return fail(fmt.Errorf("extract archive: %w", extractErr))
	}
	if closeErr != nil {
		return fail(fmt.Errorf("close archive: %w", closeErr))
	}
	exe, err := os.Lstat(filepath.Join(staging, rel))
	if err != nil {
		return fail(fmt.Errorf("archive has no %s: %w", rel, err))
	}
	if !exe.Mode().IsRegular() {
		return fail(fmt.Errorf("archive entry %s is not a regular file", rel))
	}
	return staging, nil
}

// swapFolders moves Target aside and the staging folder into its place. A failure after the first move is
// rolled back: the previous install is moved back. A rollback failure is reported with the backup path.
func swapFolders(staging string, opts ApplyOptions) error {
	backup := opts.Target + backupSuffix
	if _, err := os.Lstat(backup); err == nil {
		return fmt.Errorf("previous install %s still exists: run CleanupPreviousInstall first", backup)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", backup, err)
	}
	if err := opts.Rename(opts.Target, backup); err != nil {
		return fmt.Errorf("move current install aside: %w", err)
	}
	if err := opts.Rename(staging, opts.Target); err != nil {
		moveErr := fmt.Errorf("move new install into place: %w", err)
		if rollbackErr := opts.Rename(backup, opts.Target); rollbackErr != nil {
			return errors.Join(moveErr, fmt.Errorf("restore previous install from %s: %w", backup, rollbackErr))
		}
		opts.Log(fmt.Sprintf("[update] rolled back: %v", moveErr))
		return moveErr
	}
	return nil
}

// executableRel returns the executable path relative to the install target. It must stay inside it.
func executableRel(target string, executable string) (string, error) {
	if !filepath.IsAbs(target) {
		return "", fmt.Errorf("install target %q must be absolute", target)
	}
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("executable %q must be absolute", executable)
	}
	rel, err := filepath.Rel(target, executable)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("executable %s is not inside install target %s", executable, target)
	}
	return rel, nil
}

// verifyFileSHA256 reports an error unless the file's digest equals want.
func verifyFileSHA256(path string, want string) error {
	got, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("archive checksum mismatch: got %s, want %s", got, want)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }() // read-only handle: the close error carries no information
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// waitForProcessExit blocks until pid exited or ctx ends. On Unix it polls with signal 0. On Windows it waits
// on the process handle; if ctx ends first, that wait goroutine stays blocked until the process exits, and
// the helper process ends right after the error is returned.
func waitForProcessExit(ctx context.Context, pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		// Windows reports a process that no longer exists from FindProcess.
		return nil
	}
	if runtime.GOOS == "windows" {
		done := make(chan error, 1)
		go func() {
			_, waitErr := proc.Wait()
			done <- waitErr
		}()
		select {
		case <-done: //nolint:errcheck // the exit status of the app is irrelevant: it only has to be gone
			return nil
		case <-ctx.Done():
			return fmt.Errorf("process %d is still running: %w", pid, ctx.Err())
		}
	}
	defer func() { _ = proc.Release() }() // best effort: the helper process ends right after
	ticker := time.NewTicker(exitPollInterval)
	defer ticker.Stop()
	for {
		if !unixProcessAlive(proc) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("process %d is still running: %w", pid, ctx.Err())
		case <-ticker.C:
		}
	}
}

// unixProcessAlive reports whether signal 0 can reach the process. EPERM means it exists but belongs to
// someone else, so it is still alive.
func unixProcessAlive(proc *os.Process) bool {
	err := proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}

func archiveAssetName(version string, goos string, goarch string) string {
	return fmt.Sprintf("iworkflow-%s-%s-%s.tar.gz", version, goos, goarch)
}

func findAsset(assets []releaseAsset, name string) (releaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name && asset.BrowserDownloadURL != "" {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func isSHA256Hex(text string) bool {
	if len(text) != sha256HexLength {
		return false
	}
	_, err := hex.DecodeString(text)
	return err == nil
}

// checkStatus reports a non-2xx answer as an error.
func checkStatus(resp *http.Response) error {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GitHub answered %d", resp.StatusCode)
	}
	return nil
}

// removeLogged deletes a file, logging failures other than "does not exist".
func removeLogged(path string, logf func(string)) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logf(fmt.Sprintf("[update] remove %s: %v", path, err))
	}
}

// removeAllLogged deletes a tree, logging failures.
func removeAllLogged(path string, logf func(string)) {
	if err := os.RemoveAll(path); err != nil {
		logf(fmt.Sprintf("[update] remove %s: %v", path, err))
	}
}

const (
	renameAttempts = 20
	renameDelay    = 500 * time.Millisecond
)

// renameWithRetry moves a folder, retrying for a few seconds: on Windows, antivirus scans and the web view
// of the app that just closed keep the install folder locked for a moment after the process ends.
func renameWithRetry(oldpath string, newpath string) error {
	var err error
	for attempt := 0; attempt < renameAttempts; attempt++ {
		if err = os.Rename(oldpath, newpath); err == nil {
			return nil
		}
		time.Sleep(renameDelay)
	}
	return fmt.Errorf("renaming %s after %d attempts: %w", oldpath, renameAttempts, err)
}
