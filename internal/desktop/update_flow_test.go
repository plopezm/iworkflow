package desktop

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	releaseURL     = "https://github.com/x/y/releases/tag/v0.17.0"
	newerRelease   = `{"tag_name":"v0.17.0","html_url":"https://github.com/x/y/releases/tag/v0.17.0","draft":false,"prerelease":false}`
	sameRelease    = `{"tag_name":"v0.16.0","html_url":"https://github.com/x/y/releases/tag/v0.16.0","draft":false,"prerelease":false}`
	preRelease     = `{"tag_name":"v0.18.0","html_url":"https://github.com/x/y/releases/tag/v0.18.0","draft":false,"prerelease":true}`
	testAppVersion = "0.16.0"
)

// fakeAsker answers dialogs from a queue of button indexes. An empty queue answers 0, as Electron does.
type fakeAsker struct {
	answers []int
	asked   []Question
	err     error
}

func (a *fakeAsker) ask(_ context.Context, question Question) (int, error) {
	a.asked = append(a.asked, question)
	if a.err != nil {
		return 0, a.err
	}
	if len(a.answers) == 0 {
		return 0, nil
	}
	answer := a.answers[0]
	a.answers = a.answers[1:]
	return answer, nil
}

// fakeInstaller is an Installer whose behaviour is set by each test.
type fakeInstaller struct {
	available   bool
	version     string
	checkFn     func(ctx context.Context) (UpdateCheck, error)
	downloadErr error
	quitErr     error
	quitCalls   int
	downloads   int
	prepares    int
	prepareErr  error
	onFailure   func(error)
	onQuit      func()
}

func (f *fakeInstaller) Check(ctx context.Context) (UpdateCheck, error) {
	if f.checkFn != nil {
		return f.checkFn(ctx)
	}
	return UpdateCheck{Available: f.available, Version: f.version}, nil
}

func (f *fakeInstaller) Download(_ context.Context, progress func(int64, int64)) error {
	f.downloads++
	if progress != nil {
		progress(5, 10)
	}
	return f.downloadErr
}

func (f *fakeInstaller) Prepare(context.Context) error {
	f.prepares++
	return f.prepareErr
}

func (f *fakeInstaller) QuitAndInstall(_ context.Context, onFailure func(error)) error {
	f.quitCalls++
	f.onFailure = onFailure
	if f.onQuit != nil {
		f.onQuit()
	}
	return f.quitErr
}

// fixture bundles a Flow with the fakes it reports to.
type fixture struct {
	flow      *Flow
	asker     *fakeAsker
	inst      *fakeInstaller
	opened    []string
	logs      []string
	events    []string
	recovered int
}

// newFixture builds an automatic flow with a newer version available. mutate may adjust the Config.
func newFixture(t *testing.T, mutate func(*Config)) *fixture {
	t.Helper()
	fx := &fixture{
		asker: &fakeAsker{},
		inst:  &fakeInstaller{available: true, version: "0.17.0"},
	}
	fx.inst.onQuit = func() { fx.events = append(fx.events, "quit") }
	cfg := Config{
		Strategy:     StrategyAuto,
		Repo:         "x/y",
		AppVersion:   testAppVersion,
		Ask:          fx.asker.ask,
		OpenExternal: func(_ context.Context, url string) error { fx.opened = append(fx.opened, url); return nil },
		Installer:    fx.inst,
		BeforeInstall: func(context.Context) error {
			fx.events = append(fx.events, "before-install")
			return nil
		},
		Recover: func() { fx.recovered++ },
		Log:     func(message string) { fx.logs = append(fx.logs, message) },
	}
	if mutate != nil {
		mutate(&cfg)
	}
	flow, err := NewFlow(cfg)
	require.NoError(t, err)
	fx.flow = flow
	return fx
}

// releaseAPI is an httptest GitHub API that records the request headers it received.
type releaseAPI struct {
	srv     *httptest.Server
	mu      sync.Mutex
	headers []http.Header
}

func newReleaseAPI(t *testing.T, status int, body string) *releaseAPI {
	t.Helper()
	api := &releaseAPI{}
	api.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		api.headers = append(api.headers, r.Header.Clone())
		api.mu.Unlock()
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(api.srv.Close)
	return api
}

func (a *releaseAPI) url() string { return a.srv.URL + "/releases/latest" }

func (a *releaseAPI) requests() []http.Header {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]http.Header(nil), a.headers...)
}

// notifyWith returns a Config mutator for the notify strategy against api.
func notifyWith(api *releaseAPI) func(*Config) {
	return func(cfg *Config) {
		cfg.Strategy = StrategyNotify
		cfg.APIURL = api.url()
	}
}

func lastQuestion(t *testing.T, asker *fakeAsker) Question {
	t.Helper()
	require.NotEmpty(t, asker.asked, "expected at least one dialog")
	return asker.asked[len(asker.asked)-1]
}

func TestAutomaticCheckThatFindsNothingStaysQuietAndManualSaysSo(t *testing.T) {
	fx := newFixture(t, nil)
	fx.inst.available = false

	require.NoError(t, fx.flow.Check(context.Background(), false))
	assert.Empty(t, fx.asker.asked)

	require.NoError(t, fx.flow.Check(context.Background(), true))
	require.Len(t, fx.asker.asked, 1)
	assert.Equal(t, "iworkflow 0.16.0 is up to date.", fx.asker.asked[0].Message)
}

func TestOneAcceptanceDownloadsPreparesAndInstallsWithProgress(t *testing.T) {
	var phases []ProgressPhase
	fx := newFixture(t, func(cfg *Config) {
		cfg.Progress = func(p Progress) { phases = append(phases, p.Phase) }
	})
	fx.asker.answers = []int{0}

	require.NoError(t, fx.flow.Check(context.Background(), false))

	require.Len(t, fx.asker.asked, 1, "a single question covers download and install")
	assert.Equal(t, []string{"Update now", "Later"}, fx.asker.asked[0].Buttons)
	assert.Equal(t, []ProgressPhase{PhaseDownloading, PhaseDownloading, PhaseInstalling}, phases)
	assert.Equal(t, 1, fx.inst.prepares)
	assert.Equal(t, []string{"before-install", "quit"}, fx.events)
}

func TestDecliningTheUpdateStartsNothing(t *testing.T) {
	fx := newFixture(t, nil)
	fx.asker.answers = []int{1}

	require.NoError(t, fx.flow.Check(context.Background(), false))

	assert.Equal(t, 0, fx.inst.downloads)
	assert.Equal(t, 0, fx.inst.prepares)
	assert.Empty(t, fx.events)
}

func TestPrepareFailureKeepsTheServerRunningAndTellsTheUser(t *testing.T) {
	var ended int
	fx := newFixture(t, func(cfg *Config) {
		cfg.Progress = func(p Progress) {
			if p.Phase == PhaseEnded {
				ended++
			}
		}
	})
	fx.inst.prepareErr = errors.New("disk full")
	fx.asker.answers = []int{0}

	require.NoError(t, fx.flow.Check(context.Background(), false))

	assert.Equal(t, 1, ended)
	assert.Empty(t, fx.events, "the server is not stopped")
	assert.Equal(t, 0, fx.inst.quitCalls)
	last := lastQuestion(t, fx.asker)
	assert.Equal(t, QuestionError, last.Type)
	assert.Equal(t, "Could not install iworkflow 0.17.0.", last.Message)
}

func TestDeclinedVersionIsNotOfferedAgainByAutomaticChecksButManualOffersIt(t *testing.T) {
	fx := newFixture(t, nil)
	fx.asker.answers = []int{1, 1}
	ctx := context.Background()

	require.NoError(t, fx.flow.Check(ctx, false))
	require.NoError(t, fx.flow.Check(ctx, false))
	assert.Len(t, fx.asker.asked, 1)

	require.NoError(t, fx.flow.Check(ctx, true))
	assert.Len(t, fx.asker.asked, 2)
}

func TestFailedDownloadClosesProgressAndIsTriedAgainByTheNextAutomaticCheck(t *testing.T) {
	var ended int
	fx := newFixture(t, func(cfg *Config) {
		cfg.Progress = func(p Progress) {
			if p.Phase == PhaseEnded {
				ended++
			}
		}
	})
	fx.inst.downloadErr = errors.New("offline")
	fx.asker.answers = []int{0, 0, 0, 0}
	ctx := context.Background()

	require.NoError(t, fx.flow.Check(ctx, false))
	require.NoError(t, fx.flow.Check(ctx, false))

	assert.Equal(t, 2, fx.inst.downloads)
	assert.Equal(t, 2, ended)
}

func TestAcceptingStopsTheServerBeforeTheInstallerStarts(t *testing.T) {
	fx := newFixture(t, nil)
	fx.asker.answers = []int{0}

	require.NoError(t, fx.flow.Check(context.Background(), true))

	assert.Equal(t, []string{"before-install", "quit"}, fx.events)
	assert.Equal(t, 1, fx.inst.downloads)
	assert.Equal(t, 0, fx.recovered)
}

func TestDownloadFailureTellsTheUserAndNeverReachesTheInstaller(t *testing.T) {
	fx := newFixture(t, func(*Config) {})
	fx.inst.downloadErr = errors.New("disk full")
	fx.asker.answers = []int{0}

	require.NoError(t, fx.flow.Check(context.Background(), false))

	last := lastQuestion(t, fx.asker)
	assert.Equal(t, QuestionError, last.Type)
	assert.Equal(t, "Could not download iworkflow 0.17.0.", last.Message)
	assert.Equal(t, 0, fx.inst.quitCalls)
	assert.NotContains(t, fx.events, "before-install")
}

func TestSynchronousInstallerFailureRecoversTheAppOnce(t *testing.T) {
	fx := newFixture(t, nil)
	fx.asker.answers = []int{0}
	fx.inst.quitErr = errors.New("spawn failed")

	require.NoError(t, fx.flow.Check(context.Background(), true))

	assert.Equal(t, 1, fx.recovered)
}

func TestAsynchronousInstallerFailureRecoversTheAppOnce(t *testing.T) {
	fx := newFixture(t, nil)
	fx.asker.answers = []int{0}

	require.NoError(t, fx.flow.Check(context.Background(), true))
	require.Equal(t, 0, fx.recovered)
	require.NotNil(t, fx.inst.onFailure)

	fx.inst.onFailure(errors.New("installer exited"))
	fx.inst.onFailure(errors.New("again"))

	assert.Equal(t, 1, fx.recovered)
}

func TestBeforeInstallErrorStopsTheInstallAndIsReported(t *testing.T) {
	fx := newFixture(t, func(cfg *Config) {
		cfg.BeforeInstall = func(context.Context) error { return errors.New("server would not stop") }
	})
	fx.asker.answers = []int{0}

	err := fx.flow.Check(context.Background(), true)

	require.Error(t, err)
	assert.ErrorContains(t, err, "server would not stop")
	assert.Equal(t, 0, fx.inst.quitCalls)
	assert.Equal(t, "Could not check for updates. Try again later.", lastQuestion(t, fx.asker).Message)
}

func TestCheckThatNeverAnswersTimesOutAndFreesTheNextCheck(t *testing.T) {
	fx := newFixture(t, func(cfg *Config) { cfg.CheckTimeout = 20 * time.Millisecond })
	fx.inst.checkFn = func(ctx context.Context) (UpdateCheck, error) {
		<-ctx.Done()
		return UpdateCheck{}, ctx.Err()
	}

	err := fx.flow.Check(context.Background(), true)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Regexp(t, "timed out", fx.logs[0])
	assert.Equal(t, "Could not check for updates. Try again later.", lastQuestion(t, fx.asker).Message)

	fx.inst.checkFn = func(context.Context) (UpdateCheck, error) { return UpdateCheck{}, nil }
	require.NoError(t, fx.flow.Check(context.Background(), true))
	assert.Equal(t, "iworkflow 0.16.0 is up to date.", lastQuestion(t, fx.asker).Message)
}

func TestCheckWhileAnotherRunsReturnsImmediately(t *testing.T) {
	fx := newFixture(t, nil)
	started := make(chan struct{})
	release := make(chan struct{})
	fx.inst.checkFn = func(context.Context) (UpdateCheck, error) {
		close(started)
		<-release
		return UpdateCheck{}, nil
	}
	done := make(chan error, 1)
	go func() { done <- fx.flow.Check(context.Background(), true) }()
	<-started

	require.NoError(t, fx.flow.Check(context.Background(), true))
	close(release)
	require.NoError(t, <-done)

	assert.Len(t, fx.asker.asked, 1, "only the first check reports")
}

func TestNotifyOpensTheReleasePageOnlyWhenTheUserAccepts(t *testing.T) {
	tests := []struct {
		name    string
		answer  int
		wantURL []string
	}{
		{name: "accepted opens the release page", answer: 0, wantURL: []string{releaseURL}},
		{name: "later opens nothing", answer: 1, wantURL: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newReleaseAPI(t, http.StatusOK, newerRelease)
			fx := newFixture(t, notifyWith(api))
			fx.asker.answers = []int{tt.answer}

			require.NoError(t, fx.flow.Check(context.Background(), false))

			assert.Equal(t, tt.wantURL, fx.opened)
			assert.Equal(t, "iworkflow 0.17.0 is available.", fx.asker.asked[0].Message)
		})
	}
}

func TestNotifyDoesNotOfferTheSameDeclinedVersionTwiceAutomatically(t *testing.T) {
	api := newReleaseAPI(t, http.StatusOK, newerRelease)
	fx := newFixture(t, notifyWith(api))
	fx.asker.answers = []int{1, 1}
	ctx := context.Background()

	require.NoError(t, fx.flow.Check(ctx, false))
	require.NoError(t, fx.flow.Check(ctx, false))
	assert.Len(t, fx.asker.asked, 1)

	require.NoError(t, fx.flow.Check(ctx, true))
	assert.Len(t, fx.asker.asked, 2)
}

func TestNotifyManualCheckReportsUpToDateForSameVersionAndPreRelease(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "same version", payload: sameRelease},
		{name: "pre-release is not offered", payload: preRelease},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newReleaseAPI(t, http.StatusOK, tt.payload)
			fx := newFixture(t, notifyWith(api))

			require.NoError(t, fx.flow.Check(context.Background(), false))
			assert.Empty(t, fx.asker.asked, "automatic check stays quiet")

			require.NoError(t, fx.flow.Check(context.Background(), true))
			assert.Equal(t, "iworkflow 0.16.0 is up to date.", lastQuestion(t, fx.asker).Message)
			assert.Empty(t, fx.opened)
		})
	}
}

func TestNotifyNetworkErrorsAreQuietForAutomaticChecksAndReportedForManualOnes(t *testing.T) {
	api := newReleaseAPI(t, http.StatusInternalServerError, "")
	fx := newFixture(t, notifyWith(api))

	// The failure is returned and logged, but an automatic check shows nothing.
	require.ErrorContains(t, fx.flow.Check(context.Background(), false), "GitHub answered 500")
	assert.Empty(t, fx.asker.asked)
	assert.Contains(t, fx.logs[0], "GitHub answered 500")

	err := fx.flow.Check(context.Background(), true)
	require.Error(t, err)
	require.Len(t, fx.asker.asked, 1)
	assert.Equal(t, "Could not check for updates. Try again later.", fx.asker.asked[0].Message)
}

func TestNotifySendsGitHubHeadersAndTokenOnlyWhenConfigured(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantAuth  string
		wantAgent string
	}{
		{name: "public repository without token", token: "", wantAuth: "", wantAgent: userAgent},
		{name: "private repository with token", token: "tok", wantAuth: "Bearer tok", wantAgent: userAgent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newReleaseAPI(t, http.StatusOK, sameRelease)
			fx := newFixture(t, func(cfg *Config) {
				notifyWith(api)(cfg)
				cfg.GHToken = tt.token
			})

			require.NoError(t, fx.flow.Check(context.Background(), true))

			reqs := api.requests()
			require.Len(t, reqs, 1)
			assert.Equal(t, "application/vnd.github+json", reqs[0].Get("Accept"))
			assert.Equal(t, tt.wantAgent, reqs[0].Get("User-Agent"))
			assert.Equal(t, tt.wantAuth, reqs[0].Get("Authorization"))
		})
	}
}

func TestAutoStrategyWithoutInstallerFallsBackToNotify(t *testing.T) {
	api := newReleaseAPI(t, http.StatusOK, newerRelease)
	fx := newFixture(t, func(cfg *Config) {
		notifyWith(api)(cfg)
		cfg.Strategy = StrategyAuto
		cfg.Installer = nil
	})
	fx.asker.answers = []int{0}

	require.NoError(t, fx.flow.Check(context.Background(), false))

	assert.Equal(t, []string{releaseURL}, fx.opened)
	assert.Equal(t, "iworkflow 0.17.0 is available.", fx.asker.asked[0].Message)
}

func TestOffStrategyDoesNothing(t *testing.T) {
	fx := newFixture(t, func(cfg *Config) { cfg.Strategy = StrategyOff })

	require.NoError(t, fx.flow.Check(context.Background(), true))
	assert.Empty(t, fx.asker.asked)

	require.NoError(t, fx.flow.Run(context.Background()))
	assert.Empty(t, fx.asker.asked)
}

func TestRunChecksAfterTheFirstDelayAndStopsOnCancel(t *testing.T) {
	checked := make(chan struct{}, 10)
	fx := newFixture(t, func(cfg *Config) {
		cfg.FirstCheckDelay = 10 * time.Millisecond
		cfg.CheckInterval = time.Hour
	})
	fx.inst.checkFn = func(context.Context) (UpdateCheck, error) {
		checked <- struct{}{}
		return UpdateCheck{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- fx.flow.Run(ctx) }()

	select {
	case <-checked:
	case <-time.After(5 * time.Second):
		t.Fatal("first check did not run")
	}
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
	assert.Len(t, checked, 0, "no second check within the interval")
}

func TestNewFlowRejectsInvalidConfig(t *testing.T) {
	valid := func() Config {
		return Config{
			Strategy:     StrategyNotify,
			AppVersion:   testAppVersion,
			Ask:          (&fakeAsker{}).ask,
			OpenExternal: func(context.Context, string) error { return nil },
		}
	}
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "unknown strategy", mutate: func(cfg *Config) { cfg.Strategy = "maybe" }},
		{name: "missing app version", mutate: func(cfg *Config) { cfg.AppVersion = "" }},
		{name: "missing dialog", mutate: func(cfg *Config) { cfg.Ask = nil }},
		{name: "missing browser opener", mutate: func(cfg *Config) { cfg.OpenExternal = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(&cfg)
			_, err := NewFlow(cfg)
			assert.Error(t, err)
		})
	}
}

func TestNewFlowAppliesDefaults(t *testing.T) {
	flow, err := NewFlow(Config{
		Strategy:     StrategyNotify,
		AppVersion:   testAppVersion,
		Ask:          (&fakeAsker{}).ask,
		OpenExternal: func(context.Context, string) error { return nil },
	})
	require.NoError(t, err)

	assert.Equal(t, UpdateRepo, flow.cfg.Repo)
	assert.Equal(t, LatestReleaseAPIURL(UpdateRepo), flow.cfg.APIURL)
	assert.Equal(t, DefaultCheckTimeout, flow.cfg.CheckTimeout)
	assert.Equal(t, DefaultFirstCheckDelay, flow.cfg.FirstCheckDelay)
	assert.Equal(t, DefaultCheckInterval, flow.cfg.CheckInterval)
	assert.Equal(t, APIRequestTimeout, flow.client.Timeout)
	assert.IsType(t, UnsupportedInstaller{}, flow.cfg.Installer)
}

func TestUnsupportedInstallerReportsUnsupported(t *testing.T) {
	var inst Installer = UnsupportedInstaller{}

	_, err := inst.Check(context.Background())
	assert.ErrorIs(t, err, ErrAutoUpdateUnsupported)
	assert.ErrorIs(t, inst.Download(context.Background(), nil), ErrAutoUpdateUnsupported)
	assert.ErrorIs(t, inst.QuitAndInstall(context.Background(), func(error) {}), ErrAutoUpdateUnsupported)
}
