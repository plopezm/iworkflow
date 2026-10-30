package preferences_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/preferences/infrastructure/di"
)

// downstream records what the wrapped page saw and optionally sets its own cookie.
type downstream struct {
	cookieHeader string
	ownCookie    string
	writeBody    bool
}

func (d *downstream) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.cookieHeader = r.Header.Get("Cookie")
		if d.ownCookie != "" {
			w.Header().Add("Set-Cookie", d.ownCookie)
		}
		if d.writeBody {
			_, _ = io.WriteString(w, "page")
		}
	})
}

func runLanguageCookie(t *testing.T, module *di.Module, page *downstream, accept string, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := inbound.SyncLanguageCookie(logger, module.GetLanguage, module.SaveLanguage)(page.handler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	return rec
}

func storeLanguage(t *testing.T, module *di.Module, code string) {
	t.Helper()
	_, err := module.SaveLanguage.Execute(context.Background(), code)
	require.NoError(t, err)
}

func TestStoredLanguageIsRestoredIntoAFreshBrowser(t *testing.T) {
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})
	storeLanguage(t, module, "de")
	page := &downstream{writeBody: true}

	rec := runLanguageCookie(t, module, page, "text/html", "")

	require.Equal(t, "iw-lang=de", page.cookieHeader)
	require.Equal(t, []string{"iw-lang=de; Path=/; Max-Age=31536000; SameSite=Lax"}, rec.Header().Values("Set-Cookie"))
	require.Equal(t, "page", rec.Body.String())
}

func TestStoredLanguageReplacesADifferentCookieAndKeepsTheOthers(t *testing.T) {
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})
	storeLanguage(t, module, "de")
	page := &downstream{}

	runLanguageCookie(t, module, page, "text/html", "a=1; iw-lang=es; b=2")

	require.Equal(t, "a=1; b=2; iw-lang=de", page.cookieHeader)
}

func TestCookieChosenBeforeStorageIsPersisted(t *testing.T) {
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})
	page := &downstream{}

	rec := runLanguageCookie(t, module, page, "text/html", "iw-lang=fr")

	stored, err := module.GetLanguage.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, "fr", stored.Code())
	require.Empty(t, rec.Header().Values("Set-Cookie"), "the cookie already says fr")
	require.Equal(t, "iw-lang=fr", page.cookieHeader)
}

func TestMatchingCookieAndStoredLanguageNeedNothing(t *testing.T) {
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})
	storeLanguage(t, module, "zh")
	page := &downstream{}

	rec := runLanguageCookie(t, module, page, "text/html", "iw-lang=zh")

	require.Empty(t, rec.Header().Values("Set-Cookie"))
	require.Equal(t, "iw-lang=zh", page.cookieHeader)
}

func TestNonPageRequestsAreLeftAlone(t *testing.T) {
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})
	storeLanguage(t, module, "de")
	page := &downstream{}

	rec := runLanguageCookie(t, module, page, "application/json", "iw-lang=fr")

	require.Equal(t, "iw-lang=fr", page.cookieHeader)
	require.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestCookieIsSetEvenWhenThePageWritesNothing(t *testing.T) {
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})
	storeLanguage(t, module, "fr")
	page := &downstream{writeBody: false}

	rec := runLanguageCookie(t, module, page, "text/html", "")

	require.Equal(t, []string{"iw-lang=fr; Path=/; Max-Age=31536000; SameSite=Lax"}, rec.Header().Values("Set-Cookie"))
}

func TestLanguageCookieIsAppendedAfterTheHandlersOwnCookies(t *testing.T) {
	module := di.NewModule(di.Deps{DB: dbtest.New(t)})
	storeLanguage(t, module, "es")
	page := &downstream{ownCookie: "session=1; Path=/", writeBody: true}

	rec := runLanguageCookie(t, module, page, "text/html", "")

	require.Equal(t, []string{
		"session=1; Path=/",
		"iw-lang=es; Path=/; Max-Age=31536000; SameSite=Lax",
	}, rec.Header().Values("Set-Cookie"))
}

func TestStorageFailureLetsTheRequestThroughWithTheCookieAsItIs(t *testing.T) {
	db := dbtest.New(t)
	module := di.NewModule(di.Deps{DB: db})
	require.NoError(t, db.Close())
	page := &downstream{writeBody: true}

	rec := runLanguageCookie(t, module, page, "text/html", "iw-lang=de")

	require.Equal(t, "iw-lang=de", page.cookieHeader)
	require.Empty(t, rec.Header().Values("Set-Cookie"))
	require.Equal(t, "page", rec.Body.String())
	require.Equal(t, http.StatusOK, rec.Code)
}
