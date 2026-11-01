package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
)

type pruneFake struct {
	off    bool
	getErr error
	setErr error
}

func (f *pruneFake) Enabled(context.Context) (bool, error) { return !f.off, f.getErr }

func (f *pruneFake) SetEnabled(_ context.Context, enabled bool) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.off = !enabled
	return nil
}

func doPrune(t *testing.T, fake *pruneFake, method string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	NewPruneSettingHandlers(fake).Mount(router)
	req := httptest.NewRequest(method, "/tasks/prune-merged", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(inLanguage(i18n.En))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPruneSettingShowsTheCheckboxCheckedByDefault(t *testing.T) {
	rec := doPrune(t, &pruneFake{}, http.MethodGet, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "checked")
	assert.Contains(t, rec.Body.String(), "Delete merged tasks after one week")
}

func TestPruneSettingPostWithoutTheFieldSwitchesItOff(t *testing.T) {
	fake := &pruneFake{}

	rec := doPrune(t, fake, http.MethodPost, url.Values{})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, fake.off)
	assert.NotContains(t, rec.Body.String(), "checked")
}

func TestPruneSettingPostWithTheFieldSwitchesItOn(t *testing.T) {
	fake := &pruneFake{off: true}

	rec := doPrune(t, fake, http.MethodPost, url.Values{"enabled": {"true"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, fake.off)
	assert.Contains(t, rec.Body.String(), "checked")
}

func TestPruneSettingFailedSaveKeepsTheStoredValueAndShowsTheError(t *testing.T) {
	fake := &pruneFake{setErr: errors.New("disk full")}

	rec := doPrune(t, fake, http.MethodPost, url.Values{})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, fake.off)
	assert.Contains(t, rec.Body.String(), "disk full")
	assert.Contains(t, rec.Body.String(), "checked")
}
