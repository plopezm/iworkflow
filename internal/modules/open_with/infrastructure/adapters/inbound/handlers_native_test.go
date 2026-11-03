package inbound_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/open_with/application"
	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/inbound"
)

type stubPicker struct {
	path string
	err  error
}

func (s stubPicker) Pick(context.Context) (string, error) { return s.path, s.err }

type emptyRepo struct{}

func (emptyRepo) List(context.Context) ([]models.Launcher, error) { return nil, nil }
func (emptyRepo) Get(context.Context, string) (models.Launcher, error) {
	return models.Launcher{}, openerrors.ErrLauncherNotFound
}
func (emptyRepo) Save(context.Context, models.Launcher) error { return nil }
func (emptyRepo) Delete(context.Context, string) error        { return nil }

type emptyBrowser struct{}

func (emptyBrowser) Browse(context.Context, string) (models.Listing, error) {
	return models.Listing{Path: "/home/x"}, nil
}

func nativeRequest(t *testing.T, picker stubPicker, query string) string {
	t.Helper()
	handlers := inbound.NewHandlers(inbound.UseCases{
		List:   application.NewListLaunchers(emptyRepo{}),
		Browse: application.NewBrowseFiles(emptyBrowser{}),
		Pick:   application.NewPickExecutable(picker),
	})
	router := chi.NewRouter()
	handlers.Mount(router)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/open-with/tasks/t1/native"+query, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestNativeFillsThePathWithTheChosenFile(t *testing.T) {
	body := nativeRequest(t, stubPicker{path: "/usr/bin/code"}, "?name=Code")
	require.Contains(t, body, `value="/usr/bin/code"`)
	require.Contains(t, body, `value="Code"`)
}

func TestNativeCancelKeepsTheTypedPath(t *testing.T) {
	body := nativeRequest(t, stubPicker{}, "?path=/typed")
	require.Contains(t, body, `value="/typed"`)
}

func TestNativeFallsBackToTheWebPicker(t *testing.T) {
	body := nativeRequest(t, stubPicker{err: openerrors.ErrNativePickerUnavailable}, "")
	require.Contains(t, body, "data-open-with-picker")
}
