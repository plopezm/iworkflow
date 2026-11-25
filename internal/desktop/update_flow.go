package desktop

// The update check flow, mirroring electron/src/update-flow.js and electron/src/updater.js.
// Every side effect (dialogs, browser, installer, recovery, logging, HTTP) is injected through Config,
// so the flow runs without a GUI. Decisions live in update_policy.go.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// DefaultFirstCheckDelay is how long after Run starts the first check happens.
	DefaultFirstCheckDelay = 15 * time.Second
	// DefaultCheckInterval is the period between scheduled checks.
	DefaultCheckInterval = 6 * time.Hour
	// DefaultCheckTimeout bounds one installer check (automatic strategy).
	DefaultCheckTimeout = 60 * time.Second
	// APIRequestTimeout bounds one request to the GitHub API.
	APIRequestTimeout = 15 * time.Second

	userAgent          = "iworkflow-desktop"
	maxReleaseBodySize = 1 << 20
	msgUpToDate        = "iworkflow %s is up to date."
	msgCheckFailed     = "Could not check for updates. Try again later."
)

// ErrAutoUpdateUnsupported is returned by an Installer that cannot install updates on this build.
// The flow then falls back to the notify behaviour.
var ErrAutoUpdateUnsupported = errors.New("automatic update is not supported on this build")

// QuestionType is the kind of dialog shown to the user.
type QuestionType string

const (
	// QuestionInfo is an informational dialog.
	QuestionInfo QuestionType = "info"
	// QuestionError is an error dialog.
	QuestionError QuestionType = "error"
)

// Question describes a dialog. Buttons is empty for a message the user only acknowledges.
type Question struct {
	Type      QuestionType
	Message   string
	Detail    string
	Buttons   []string
	DefaultID int
	CancelID  int
}

// Asker shows a dialog and returns the index of the button the user chose. Dialogs without buttons
// return 0; the caller ignores the value.
type Asker func(ctx context.Context, question Question) (int, error)

// UpdateCheck is the result of asking an Installer whether a newer version exists.
type UpdateCheck struct {
	Available bool
	Version   string
}

// Installer performs automatic updates. It is only used with StrategyAuto.
type Installer interface {
	// Check asks whether a newer version is available.
	Check(ctx context.Context) (UpdateCheck, error)
	// Download fetches the update. progress, when not nil, receives the bytes read so far and the total
	// size (-1 when unknown).
	Download(ctx context.Context, progress func(done int64, total int64)) error
	// Prepare unpacks the downloaded update while the app still runs. A failure leaves the app untouched.
	Prepare(ctx context.Context) error
	// QuitAndInstall starts the installer and ends the app. It returns an error when the installer did
	// not start. onFailure may also be called later, from another goroutine, when the installer fails
	// after it started; the flow calls Config.Recover at most once per attempt either way.
	QuitAndInstall(ctx context.Context, onFailure func(error)) error
}

// UnsupportedInstaller is the default Installer. Every method returns ErrAutoUpdateUnsupported, so an
// automatic strategy without a real installer behaves like notify.
type UnsupportedInstaller struct{}

// Check always reports ErrAutoUpdateUnsupported.
func (UnsupportedInstaller) Check(context.Context) (UpdateCheck, error) {
	return UpdateCheck{}, ErrAutoUpdateUnsupported
}

// Download always reports ErrAutoUpdateUnsupported.
func (UnsupportedInstaller) Download(context.Context, func(int64, int64)) error {
	return ErrAutoUpdateUnsupported
}

// Prepare always reports ErrAutoUpdateUnsupported.
func (UnsupportedInstaller) Prepare(context.Context) error {
	return ErrAutoUpdateUnsupported
}

// QuitAndInstall always reports ErrAutoUpdateUnsupported.
func (UnsupportedInstaller) QuitAndInstall(context.Context, func(error)) error {
	return ErrAutoUpdateUnsupported
}

// ProgressPhase is the stage of an automatic update shown to the user.
type ProgressPhase string

const (
	// PhaseDownloading means the archive is being downloaded and verified.
	PhaseDownloading ProgressPhase = "downloading"
	// PhaseInstalling means the server is stopped and the installer is being started.
	PhaseInstalling ProgressPhase = "installing"
	// PhaseEnded means the update did not go on (it failed or was cancelled), so the progress view closes.
	PhaseEnded ProgressPhase = "ended"
)

// Progress is one step of an automatic update. Total is -1 when the size is unknown.
type Progress struct {
	Phase   ProgressPhase
	Version string
	Done    int64
	Total   int64
}

// Config holds the strategy and every injected dependency of a Flow.
type Config struct {
	// Strategy is auto, notify or off. Off makes Check and Run do nothing.
	Strategy Strategy
	// Repo is the "owner/name" GitHub repository. Empty means UpdateRepo.
	Repo string
	// APIURL overrides the latest-release URL. Empty means LatestReleaseAPIURL(Repo).
	APIURL string
	// AppVersion is the running version, for example "0.16.0". Required.
	AppVersion string
	// HTTPClient performs the GitHub API request. Nil means a client with APIRequestTimeout.
	HTTPClient *http.Client
	// GHToken, when set, is sent as a Bearer token (needed for private repositories). Never logged.
	GHToken string
	// Ask shows dialogs. Required.
	Ask Asker
	// OpenExternal opens a URL in the system browser. Required.
	OpenExternal func(ctx context.Context, url string) error
	// Progress shows the download and the install in the foreground. Optional.
	Progress func(Progress)
	// Installer performs automatic updates. Nil means UnsupportedInstaller.
	Installer Installer
	// BeforeInstall stops whatever would block the installer, such as the server process.
	BeforeInstall func(ctx context.Context) error
	// Recover brings the app back when the installer could not start. Optional.
	Recover func()
	// Log receives diagnostic messages. Optional.
	Log func(message string)
	// CheckTimeout bounds one installer check. Zero means DefaultCheckTimeout.
	CheckTimeout time.Duration
	// FirstCheckDelay is the delay before the first scheduled check. Zero means DefaultFirstCheckDelay.
	FirstCheckDelay time.Duration
	// CheckInterval is the period of scheduled checks. Zero means DefaultCheckInterval.
	CheckInterval time.Duration
}

// Flow runs update checks. It is safe for concurrent use: a check started while another one runs returns
// immediately, as the Electron flow did.
type Flow struct {
	cfg      Config
	client   *http.Client
	mu       sync.Mutex
	busy     bool
	declined string
	cancel   context.CancelFunc
}

// NewFlow validates cfg, applies defaults and returns a Flow.
func NewFlow(cfg Config) (*Flow, error) {
	switch cfg.Strategy {
	case StrategyAuto, StrategyNotify, StrategyOff:
	default:
		return nil, fmt.Errorf("unknown update strategy %q", cfg.Strategy)
	}
	if cfg.AppVersion == "" {
		return nil, errors.New("update flow needs the application version")
	}
	if cfg.Ask == nil {
		return nil, errors.New("update flow needs an Ask dialog")
	}
	if cfg.OpenExternal == nil {
		return nil, errors.New("update flow needs an OpenExternal function")
	}
	if cfg.Repo == "" {
		cfg.Repo = UpdateRepo
	}
	if cfg.APIURL == "" {
		cfg.APIURL = LatestReleaseAPIURL(cfg.Repo)
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: APIRequestTimeout}
	}
	if cfg.Installer == nil {
		cfg.Installer = UnsupportedInstaller{}
	}
	if cfg.CheckTimeout <= 0 {
		cfg.CheckTimeout = DefaultCheckTimeout
	}
	if cfg.FirstCheckDelay <= 0 {
		cfg.FirstCheckDelay = DefaultFirstCheckDelay
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = DefaultCheckInterval
	}
	return &Flow{cfg: cfg, client: cfg.HTTPClient}, nil
}

// Run performs the first check after FirstCheckDelay and then one every CheckInterval. It blocks until
// ctx is cancelled and returns nil then. The caller owns the goroutine that runs it. It returns at once
// when the strategy is off.
func (f *Flow) Run(ctx context.Context) error {
	if f.cfg.Strategy == StrategyOff {
		return nil
	}
	ticker := time.NewTicker(f.cfg.CheckInterval)
	defer ticker.Stop()
	first := time.NewTimer(f.cfg.FirstCheckDelay)
	defer first.Stop()

	select {
	case <-ctx.Done():
		return nil
	case <-first.C:
	}
	f.scheduled(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			f.scheduled(ctx)
		}
	}
}

// scheduled runs one automatic check. Check logs and reports its own failures, so nothing is left to do.
func (f *Flow) scheduled(ctx context.Context) {
	// The error is already logged, and automatic checks stay silent for the user.
	_ = f.Check(ctx, false)
}

// Check runs one update check. A manual check (requested by the user) always reports its outcome; an
// automatic one stays quiet unless an update is available and was not declined before.
//
// Failures are logged and, for a manual check, shown to the user. The returned error is informational.
func (f *Flow) Check(ctx context.Context, manual bool) error {
	if f.cfg.Strategy == StrategyOff {
		return nil
	}
	if !f.acquire() {
		return nil
	}
	defer f.release()

	var err error
	if f.cfg.Strategy == StrategyAuto {
		err = f.installAutomatically(ctx, manual)
		if errors.Is(err, ErrAutoUpdateUnsupported) {
			f.log("[update] automatic update unavailable, falling back to the release notice")
			err = f.notifyOnly(ctx, manual)
		}
	} else {
		err = f.notifyOnly(ctx, manual)
	}
	if err == nil {
		return nil
	}
	f.log(fmt.Sprintf("[update] %v", err))
	reportErr := f.report(ctx, manual, msgCheckFailed)
	return errors.Join(err, reportErr)
}

// notifyOnly fetches the latest release, and if it is newer, asks the user whether to open its page.
func (f *Flow) notifyOnly(ctx context.Context, manual bool) error {
	release, ok, err := f.fetchLatestRelease(ctx)
	if err != nil {
		return err
	}
	if !ok || !IsNewerVersion(f.cfg.AppVersion, release.Version) {
		return f.upToDate(ctx, manual)
	}
	if f.consumeOffer(manual, release.Version) {
		return nil
	}
	choice, err := f.cfg.Ask(ctx, Question{
		Type:      QuestionInfo,
		Message:   fmt.Sprintf("iworkflow %s is available.", release.Version),
		Detail:    "Download the new installer from the release page and run it.",
		Buttons:   []string{"Open release page", "Later"},
		DefaultID: 0,
		CancelID:  1,
	})
	if err != nil {
		return fmt.Errorf("ask about release %s: %w", release.Version, err)
	}
	if choice == 0 {
		if err := f.cfg.OpenExternal(ctx, release.URL); err != nil {
			return fmt.Errorf("open release page: %w", err)
		}
	}
	return nil
}

// fetchLatestRelease reads the latest release from the GitHub API. The bool is false when the release is
// a draft, a pre-release or unusable.
func (f *Flow) fetchLatestRelease(ctx context.Context) (Release, bool, error) {
	reqCtx, cancel := context.WithTimeout(ctx, APIRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, f.cfg.APIURL, nil)
	if err != nil {
		return Release{}, false, fmt.Errorf("build release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	if f.cfg.GHToken != "" {
		req.Header.Set("Authorization", "Bearer "+f.cfg.GHToken)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return Release{}, false, fmt.Errorf("fetch latest release: %w", err)
	}
	defer f.closeBody(resp.Body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Release{}, false, fmt.Errorf("GitHub answered %d", resp.StatusCode)
	}
	var payload ReleasePayload
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseBodySize)).Decode(&payload); err != nil {
		return Release{}, false, fmt.Errorf("decode latest release: %w", err)
	}
	release, ok := ParseLatestRelease(&payload)
	return release, ok, nil
}

// installAutomatically runs the installer flow without questions: check, download with progress, stop
// the server, and quit into the installer. Without a real installer it returns ErrAutoUpdateUnsupported.
func (f *Flow) installAutomatically(ctx context.Context, manual bool) error {
	checkCtx, cancel := context.WithTimeout(ctx, f.cfg.CheckTimeout)
	defer cancel()
	found, err := f.cfg.Installer.Check(checkCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("update check timed out after %s: %w", f.cfg.CheckTimeout, err)
		}
		return fmt.Errorf("check for update: %w", err)
	}
	if !found.Available {
		return f.upToDate(ctx, manual)
	}
	if f.consumeOffer(manual, found.Version) {
		return nil
	}
	download, err := f.cfg.Ask(ctx, Question{
		Type:      QuestionInfo,
		Message:   fmt.Sprintf("iworkflow %s is available.", found.Version),
		Detail:    "It downloads and installs now, with a progress window, and the app restarts by itself when it finishes.",
		Buttons:   []string{"Update now", "Later"},
		DefaultID: 0,
		CancelID:  1,
	})
	if err != nil {
		return fmt.Errorf("ask to download %s: %w", found.Version, err)
	}
	if download != 0 {
		return nil
	}
	// The progress window stays visible from the download until the app quits.
	downloadCtx, cancelDownload := context.WithCancel(ctx)
	defer cancelDownload()
	f.setCancel(cancelDownload)
	defer f.setCancel(nil)
	f.progress(Progress{Phase: PhaseDownloading, Version: found.Version, Total: -1})
	err = f.cfg.Installer.Download(downloadCtx, func(done int64, total int64) {
		f.progress(Progress{Phase: PhaseDownloading, Version: found.Version, Done: done, Total: total})
	})
	// From here on the update can no longer be cancelled by the user: either it was cancelled before this
	// point, or the install is committed. Both are decided under one lock, so a close cannot fall in between.
	if f.commit(downloadCtx) && ctx.Err() == nil {
		// The user closed the progress window: that is a decision, not a failure to report.
		f.log(fmt.Sprintf("[update] update %s cancelled by the user", found.Version))
		f.progress(Progress{Phase: PhaseEnded, Version: found.Version})
		return nil
	}
	if err != nil {
		f.log(fmt.Sprintf("[update] download %s: %v", found.Version, err))
		return f.abort(ctx, found.Version, fmt.Sprintf("Could not download iworkflow %s.", found.Version))
	}
	// One acceptance covers everything: the download, the unpacking and the restart all show progress.
	f.progress(Progress{Phase: PhaseInstalling, Version: found.Version, Total: -1})
	if err := f.cfg.Installer.Prepare(ctx); err != nil {
		f.log(fmt.Sprintf("[update] prepare %s: %v", found.Version, err))
		return f.abort(ctx, found.Version, fmt.Sprintf("Could not install iworkflow %s.", found.Version))
	}
	if f.cfg.BeforeInstall != nil {
		if err := f.cfg.BeforeInstall(ctx); err != nil {
			f.progress(Progress{Phase: PhaseEnded, Version: found.Version})
			f.forget(found.Version)
			return fmt.Errorf("stop before install: %w", err)
		}
	}
	// The server is already stopped, so a failing installer must not leave the app half dead. A
	// successful install ends the app, so any failure reported from here means the install did not start.
	var once sync.Once
	failInstall := func(err error) {
		once.Do(func() {
			f.log(fmt.Sprintf("[update] install: %v", err))
			f.progress(Progress{Phase: PhaseEnded, Version: found.Version})
			f.forget(found.Version)
			if f.cfg.Recover != nil {
				f.cfg.Recover()
			}
		})
	}
	if err := f.cfg.Installer.QuitAndInstall(ctx, failInstall); err != nil {
		failInstall(err)
	}
	return nil
}

// CancelUpdate stops the download in progress and reports whether it did. Closing the progress window calls
// it. It returns false when there is nothing to cancel or the update is committed to installing: that part
// cannot be interrupted.
func (f *Flow) CancelUpdate() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancel == nil {
		return false
	}
	f.cancel()
	return true
}

func (f *Flow) setCancel(cancel context.CancelFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancel = cancel
}

// commit ends the window in which the update can be cancelled and reports whether it was cancelled in it.
func (f *Flow) commit(downloadCtx context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancel = nil
	return downloadCtx.Err() != nil
}

// abort closes the progress view, lets the next automatic check try this version again and tells the user.
func (f *Flow) abort(ctx context.Context, version string, message string) error {
	f.progress(Progress{Phase: PhaseEnded, Version: version})
	f.forget(version)
	if _, err := f.cfg.Ask(ctx, Question{Type: QuestionError, Message: message, Detail: "It is tried again automatically later."}); err != nil {
		return fmt.Errorf("show update failure: %w", err)
	}
	return nil
}

// forget makes the next automatic check offer version again.
func (f *Flow) forget(version string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.declined == version {
		f.declined = ""
	}
}

func (f *Flow) progress(p Progress) {
	if f.cfg.Progress != nil {
		f.cfg.Progress(p)
	}
}

// upToDate tells a manual caller that the installed version is current. Automatic checks stay silent.
func (f *Flow) upToDate(ctx context.Context, manual bool) error {
	return f.report(ctx, manual, fmt.Sprintf(msgUpToDate, f.cfg.AppVersion))
}

// report shows an acknowledgement dialog for a manual check only.
func (f *Flow) report(ctx context.Context, manual bool, message string) error {
	if !manual {
		return nil
	}
	if _, err := f.cfg.Ask(ctx, Question{Type: QuestionInfo, Message: message}); err != nil {
		return fmt.Errorf("show update message: %w", err)
	}
	return nil
}

// consumeOffer reports whether an automatic check must skip an offer the user already declined. It also
// records version as the latest offer, as the Electron flow did before showing the dialog.
func (f *Flow) consumeOffer(manual bool, version string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !manual && f.declined == version {
		return true
	}
	f.declined = version
	return false
}

func (f *Flow) acquire() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.busy {
		return false
	}
	f.busy = true
	return true
}

func (f *Flow) release() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.busy = false
}

func (f *Flow) closeBody(body io.Closer) {
	if err := body.Close(); err != nil {
		f.log(fmt.Sprintf("[update] close release response: %v", err))
	}
}

func (f *Flow) log(message string) {
	if f.cfg.Log != nil {
		f.cfg.Log(message)
	}
}
