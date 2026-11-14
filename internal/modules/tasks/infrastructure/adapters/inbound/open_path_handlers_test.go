package inbound

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

type stubOpen struct {
	err  error
	path string
}

func (s *stubOpen) Execute(_ context.Context, _ models.TaskID, raw string) error {
	s.path = raw
	return s.err
}

func postOpenPath(t *testing.T, uc PathOpenerUseCase, id string, path string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	NewOpenPathHandlers(uc).Mount(router)
	body := url.Values{"path": {path}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+id+"/open-path", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestOpenPathStatuses(t *testing.T) {
	id, err := models.NewTaskID()
	assert.NoError(t, err)
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"opened", nil, http.StatusNoContent},
		{"invalid", werrors.ErrOpenPathInvalid, http.StatusUnprocessableEntity},
		{"blocked", werrors.ErrOpenPathBlocked, http.StatusForbidden},
		{"missing", werrors.ErrOpenPathMissing, http.StatusNotFound},
		{"task gone", werrors.NotFound("x"), http.StatusNotFound},
		{"unknown failure", assert.AnError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := &stubOpen{err: tc.err}
			rec := postOpenPath(t, uc, id.String(), "src/a.go:3")
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, "src/a.go:3", uc.path)
			assert.NotContains(t, rec.Body.String(), assert.AnError.Error())
		})
	}
}

func TestOpenPathRejectsAnInvalidTaskID(t *testing.T) {
	uc := &stubOpen{}
	rec := postOpenPath(t, uc, "not-a-task", "a.go")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, uc.path)
}
