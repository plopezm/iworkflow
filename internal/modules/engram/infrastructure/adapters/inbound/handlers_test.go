package inbound

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
)

type fakeGet struct{ settings models.ServerSettings }

func (f *fakeGet) Execute(context.Context) (models.ServerSettings, error) { return f.settings, nil }

type fakeSave struct {
	got []string
	err error
}

func (f *fakeSave) Execute(_ context.Context, raw string) error {
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, raw)
	return nil
}

type fakeApply struct {
	calls   int
	outcome models.StartOutcome
	err     error
}

func (f *fakeApply) Execute(context.Context) (models.ServerSettings, models.StartOutcome, error) {
	f.calls++
	return models.ServerSettings{}, f.outcome, f.err
}

type fakeCheck struct{ up bool }

func (f fakeCheck) Execute(context.Context, string) bool { return f.up }

func newRouter(get *fakeGet, save *fakeSave, apply *fakeApply, up bool) http.Handler {
	h := NewHandlers(UseCases{Get: get, Save: save, Apply: apply, Check: fakeCheck{up: up}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	router := chi.NewRouter()
	h.Mount(router)
	return router
}

func do(t *testing.T, router http.Handler, method string, path string, form url.Values) string {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request := httptest.NewRequest(method, path, body)
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	request = request.WithContext(i18n.WithState(request.Context(), i18n.State{Locale: i18n.Es, Chosen: true}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Body.String()
}

var saved = models.ServerSettings{URL: "http://127.0.0.1:9000", Source: models.SourceSaved}

func TestShowRendersTheEffectiveAddressAndReachability(t *testing.T) {
	body := do(t, newRouter(&fakeGet{saved}, &fakeSave{}, &fakeApply{}, true), http.MethodGet, "/engram/settings/", nil)

	assert.Contains(t, body, `value="http://127.0.0.1:9000"`)
	assert.Contains(t, body, `placeholder="http://127.0.0.1:7437"`)
	assert.Contains(t, body, "data-engram-reachable")
	assert.Contains(t, body, "Restaurar por defecto", "a saved address can be reset")

	body = do(t, newRouter(&fakeGet{models.ServerSettings{URL: models.DefaultServerURL, Source: models.SourceDefault}}, &fakeSave{}, &fakeApply{}, false), http.MethodGet, "/engram/settings/", nil)
	assert.Contains(t, body, "data-engram-unreachable")
	assert.NotContains(t, body, "Restaurar por defecto")
}

func TestShowWarnsWhenTheAddressComesFromTheEnvironment(t *testing.T) {
	body := do(t, newRouter(&fakeGet{models.ServerSettings{URL: "http://127.0.0.1:1", Source: models.SourceEnvironment}}, &fakeSave{}, &fakeApply{}, false), http.MethodGet, "/engram/settings/", nil)
	assert.Contains(t, body, "IWF_ENGRAM_URL")
}

func TestSaveStoresAppliesAndReportsWhatHappened(t *testing.T) {
	tests := map[string]struct {
		outcome models.StartOutcome
		err     error
		want    string
	}{
		"already running": {outcome: models.OutcomeRunning, want: "ya estaba en marcha"},
		"started":         {outcome: models.OutcomeStarted, want: "se ha iniciado"},
		"skipped":         {outcome: models.OutcomeSkipped, want: "no se iniciará desde aquí"},
		"start failed":    {err: engramerrors.ErrStartFailed, want: "No se ha podido iniciar Engram"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			save, apply := &fakeSave{}, &fakeApply{outcome: tc.outcome, err: tc.err}
			body := do(t, newRouter(&fakeGet{saved}, save, apply, true), http.MethodPost, "/engram/settings/", url.Values{"url": {"http://127.0.0.1:9000"}})

			assert.Equal(t, []string{"http://127.0.0.1:9000"}, save.got)
			assert.Equal(t, 1, apply.calls)
			assert.Contains(t, body, tc.want)
		})
	}
}

func TestSaveRejectsAnInvalidAddressWithoutApplying(t *testing.T) {
	save, apply := &fakeSave{err: engramerrors.ErrInvalidServerURL}, &fakeApply{}
	body := do(t, newRouter(&fakeGet{saved}, save, apply, true), http.MethodPost, "/engram/settings/", url.Values{"url": {"ftp://x"}})

	assert.Contains(t, body, "debe ser una URL http o https")
	assert.Zero(t, apply.calls)
}

func TestSaveHidesStorageErrorDetail(t *testing.T) {
	save := &fakeSave{err: errors.New("database is locked: secret")}
	body := do(t, newRouter(&fakeGet{saved}, save, &fakeApply{}, true), http.MethodPost, "/engram/settings/", url.Values{"url": {"http://127.0.0.1:9000"}})

	assert.Contains(t, body, "No se ha podido guardar")
	assert.NotContains(t, body, "secret")
}

func TestResetSavesABlankAddressAndApplies(t *testing.T) {
	save, apply := &fakeSave{}, &fakeApply{outcome: models.OutcomeRunning}
	do(t, newRouter(&fakeGet{saved}, save, apply, true), http.MethodPost, "/engram/settings/reset", nil)

	assert.Equal(t, []string{""}, save.got)
	assert.Equal(t, 1, apply.calls)
}
