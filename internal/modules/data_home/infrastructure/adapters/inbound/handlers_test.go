package inbound_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/data_home/application"
	domainerrors "iworkflow/internal/modules/data_home/domain/errors"
	"iworkflow/internal/modules/data_home/domain/models"
	"iworkflow/internal/modules/data_home/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/data_home/infrastructure/adapters/inbound/dto"
)

// fakeMover keeps the status in memory; requestErr and cancelErr make the writes fail.
type fakeMover struct {
	status     models.HomeStatus
	statusErr  error
	requestErr error
	requested  []string
}

func (f *fakeMover) Status(context.Context) (models.HomeStatus, error) {
	return f.status, f.statusErr
}

func (f *fakeMover) Request(_ context.Context, destination string) error {
	if f.requestErr != nil {
		return f.requestErr
	}
	f.requested = append(f.requested, destination)
	f.status.Pending = &models.PendingMove{From: f.status.Current, To: destination}
	return nil
}

func (f *fakeMover) Cancel(context.Context) error {
	f.status.Pending = nil
	return nil
}

type fakeTasks struct{ active bool }

func (f fakeTasks) HasActive(context.Context) (bool, error) { return f.active, nil }

type fakeRestarter struct{ requests int }

func (f *fakeRestarter) RequestRestart() { f.requests++ }

// harness is the router of the module over fake ports, with the handles the tests inspect.
type harness struct {
	router    http.Handler
	mover     *fakeMover
	restarter *fakeRestarter
}

func newHarness(t *testing.T, mover *fakeMover, tasks fakeTasks) harness {
	t.Helper()
	restarter := &fakeRestarter{}
	handlers := inbound.NewHandlers(
		application.NewGetDataHome(mover),
		application.NewRequestDataHomeMove(mover, tasks, restarter),
		application.NewCancelDataHomeMove(mover),
	)
	router := chi.NewRouter()
	handlers.Mount(router)
	return harness{router: router, mover: mover, restarter: restarter}
}

func defaultMover() *fakeMover {
	return &fakeMover{status: models.HomeStatus{Current: "/data/iwf", Source: models.SourceDefault}}
}

func (h harness) do(t *testing.T, method string, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, request)
	return rec
}

func TestCardRouteRendersTheCurrentFolder(t *testing.T) {
	rec := newHarness(t, defaultMover(), fakeTasks{}).do(t, http.MethodGet, "/data-home/card", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `id="data-home-card"`)
	require.Contains(t, rec.Body.String(), "/data/iwf")
}

func TestCardRouteShowsTheReadErrorInsteadOfTheForm(t *testing.T) {
	mover := &fakeMover{statusErr: domainerrors.ErrUnavailable}

	rec := newHarness(t, mover, fakeTasks{}).do(t, http.MethodGet, "/data-home/card", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "cannot be managed")
	require.NotContains(t, rec.Body.String(), `hx-post="/data-home/move"`)
}

func TestMoveRecordsTheDestinationAndRestartsTheServer(t *testing.T) {
	mover := defaultMover()
	h := newHarness(t, mover, fakeTasks{})

	rec := h.do(t, http.MethodPost, "/data-home/move", url.Values{"destination": {"  /mnt/iwf  "}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []string{"/mnt/iwf"}, mover.requested, "the destination is trimmed")
	require.Equal(t, 1, h.restarter.requests)
	require.Contains(t, rec.Body.String(), `hx-trigger="every 2s"`, "the card polls while the server restarts")
}

func TestMoveRefusedWhileATaskIsActiveShowsTheNoticeAndKeepsTheServer(t *testing.T) {
	mover := defaultMover()
	h := newHarness(t, mover, fakeTasks{active: true})

	rec := h.do(t, http.MethodPost, "/data-home/move", url.Values{"destination": {"/mnt/iwf"}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Stop the tasks")
	require.Empty(t, mover.requested)
	require.Zero(t, h.restarter.requests)
}

func TestMoveRefusedForItsDestinationShowsTheTranslatedReason(t *testing.T) {
	mover := defaultMover()
	mover.requestErr = &domainerrors.DestinationError{Reason: domainerrors.ReasonNotEmpty}
	h := newHarness(t, mover, fakeTasks{})

	rec := h.do(t, http.MethodPost, "/data-home/move", url.Values{"destination": {"/mnt/iwf"}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "must be a new or empty folder")
	require.Zero(t, h.restarter.requests)
}

func TestCancelWithdrawsThePendingMove(t *testing.T) {
	mover := defaultMover()
	mover.status.Pending = &models.PendingMove{From: "/data/iwf", To: "/mnt/iwf"}

	rec := newHarness(t, mover, fakeTasks{}).do(t, http.MethodPost, "/data-home/cancel", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, mover.status.Pending)
	require.NotContains(t, rec.Body.String(), "waits for the next start")
}

func TestDecodeMoveFormReadsTheTrimmedDestination(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/data-home/move", strings.NewReader("destination=+%2Fmnt%2Fiwf+"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	form, err := dto.DecodeMoveForm(request)

	require.NoError(t, err)
	require.Equal(t, "/mnt/iwf", form.Destination)
}

func TestFromHomeStatusKeepsEveryFieldOfTheMoves(t *testing.T) {
	status := models.HomeStatus{
		Current:   "/mnt/iwf",
		Source:    models.SourcePointer,
		EnvLocked: false,
		Pending:   &models.PendingMove{From: "/mnt/iwf", To: "/srv/iwf"},
		Last:      &models.MoveOutcome{OK: false, From: "/data/iwf", To: "/mnt/iwf", Error: "boom", Warnings: []string{"w"}},
	}

	mapped := dto.FromHomeStatus(status)

	require.Equal(t, "pointer", mapped.Source)
	require.Equal(t, dto.PendingDTO{From: "/mnt/iwf", To: "/srv/iwf"}, *mapped.Pending)
	require.False(t, mapped.Last.OK)
	require.Equal(t, "boom", mapped.Last.Error)
	require.Equal(t, []string{"w"}, mapped.Last.Warnings)
}

func TestNoticeOfMapsEveryRefusalToItsKind(t *testing.T) {
	cases := []struct {
		name string
		err  error
		kind string
	}{
		{name: "env locked", err: domainerrors.ErrEnvLocked, kind: "env_locked"},
		{name: "active tasks", err: domainerrors.ErrActiveTasks, kind: "active_tasks"},
		{name: "pending", err: domainerrors.ErrMovePending, kind: "pending"},
		{name: "unavailable", err: domainerrors.ErrUnavailable, kind: "unavailable"},
		{name: "destination", err: &domainerrors.DestinationError{Reason: domainerrors.ReasonNested}, kind: "destination"},
		{name: "other failure", err: errors.New("disk gone"), kind: "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notice := inbound.NoticeOf(tc.err)

			require.Equal(t, tc.kind, string(notice.Kind))
		})
	}
}
